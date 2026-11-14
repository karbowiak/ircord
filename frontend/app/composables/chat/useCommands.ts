import type { Message } from '../../../shared/types/chat'
import { nicknameError, nicknameLimit } from '../../../shared/utils/nickname'
import { serverViewId } from '../../stores/chat'
import { channelTarget, markdownToIrc, channelRights } from '../../../shared/utils/irc'
import { createGifCommand } from '../../../shared/utils/gif-command'
import {
  briefAwayText,
  slapAction,
  commands,
  commandAliases,
  type CommandSpec,
} from '../../../shared/utils/commands'
import { ignoreCommand } from '../../../shared/utils/ignore-command'
import { runStatusCommand } from '../../../shared/utils/status-command'

export { commands } from '../../../shared/utils/commands'
export type { CommandSpec }

/**
 * What the composer should do after `run`: `handled` clears the input, `kept` leaves it for a
 * retry, `text` means it wasn't a command and should be sent as a message.
 */
export type CommandResult = 'handled' | 'kept' | 'text'

/** `/gif` lookups in flight, conversation id → query; shared so every composer sees them. */
const gifLookups = reactive(new Map<string, string>())

/** What the composer knew when the command was submitted. */
export interface CommandContext {
  replyTo?: Message
  /** Reports the target only after a send actually used the supplied reply. */
  onReplyUsed?: (conversationId: string) => void
}

/** Runs slash commands typed in the composer. */
export function useCommands() {
  const { t } = useI18n()
  const chat = useChatStore()
  const auth = useAuthStore()
  const media = useMediaStore()
  const actions = useWorkspaceActions()
  const gif = createGifCommand({
    search: async (query) => (await klipyGifs(media.appKey, query, 1, undefined, t)).items,
    // Same path as the picker: IRC gets the URL, IRCord keeps the GIF metadata. The composer,
    // which knows what was submitted, owns clearing the draft.
    send: ({ conversationId, replyTo }, item) =>
      chat.send(item.url, replyTo, item, { conversationId }),
    share: (item, query) => klipyShare(media.appKey, item, query),
    notify: chat.notify,
    translate: (key, values) => (values ? t(key, values) : t(key)),
    lookups: gifLookups,
  })

  /**
   * Handles text starting with “/”; see `CommandResult`. “//text” sends “/text” as a message.
   */
  async function run(input: string, context: CommandContext = {}): Promise<CommandResult> {
    const match = /^\/(\S+)\s*([\s\S]*)$/.exec(input.trim())
    if (!match || match[1]!.startsWith('/')) return 'text'
    const name = commandAliases[match[1]!.toLowerCase()] ?? match[1]!.toLowerCase()
    const rest = match[2]!.trim()
    const [first = '', ...others] = rest.split(/\s+/)
    const afterFirst = rest.slice(first.length).replace(/^\s+/, '')
    const spec = commands.find((c) => c.name === name)
    const c = chat.active
    const network = chat.network
    const accountId = auth.account?.id
    const networkId = chat.networkId
    const replyTo = context.replyTo
    const currentIdentity = () => auth.account?.id === accountId && chat.networkId === networkId
    const channel = c?.kind === 'channel' && !c.party ? channelTarget(c) : ''
    const raw = (line: string) => chat.raw(line, network?.id)
    const usage = () =>
      chat.notify(
        t('workspaceUi.commands.usage', {
          command: spec!.name,
          args: spec!.args ? t(`workspaceUi.commands.args.${spec!.name}`) : '',
        }),
      )

    // Operator-only commands don't exist for anyone else, not even by name.
    if (!spec || (spec.operator && (c?.party || !channelRights('you', c, network).op))) {
      chat.notify(t('workspaceUi.commands.unknown', { command: match[1] }))
      return 'kept'
    }
    if (spec.channel && !channel) {
      chat.notify(t('workspaceUi.commands.channelOnly', { command: spec.name }))
      return 'kept'
    }
    const needs = spec.args.startsWith('<')
    if (needs && !rest) {
      usage()
      return 'kept'
    }
    try {
      switch (name) {
        case 'me':
          return chat.send(rest, undefined, undefined, { kind: 'action' }) ? 'handled' : 'kept'
        case 'slap':
          // One nick only; extra words would vanish from the slap, so keep them for a retry.
          if (others.length) return (usage(), 'kept')
          // The nick goes out as typed: `_jules_` must not turn into italic “jules”.
          return chat.send(slapAction(first), undefined, undefined, {
            kind: 'action',
            verbatim: true,
          })
            ? 'handled'
            : 'kept'
        case 'msg':
        case 'query': {
          const person = chat.memberByNick(first)
          const text = afterFirst
          if (!person) {
            if (name === 'msg' && text) {
              if (!raw(`PRIVMSG ${first} :${text}`)) return 'kept'
            } else if (nicknameError(first, nicknameLimit(network?.isupport)))
              return (chat.notify(t('workspaceUi.commands.personMissing', { name: first })), 'kept')
            else {
              const direct = await chat.direct.mutateAsync({
                nickname: first,
                networkId,
                guardSelection: true,
              })
              if (!currentIdentity() || direct.networkId !== networkId) return 'kept'
            }
            break
          }
          const direct = await chat.direct.mutateAsync({
            memberId: person.id,
            networkId,
            guardSelection: true,
          })
          if (!currentIdentity() || direct.networkId !== networkId) return 'kept'
          if (name === 'msg' && text) {
            const reply = replyTo?.conversationId === direct.id ? replyTo : undefined
            if (!chat.send(text, reply, undefined, { conversationId: direct.id })) return 'kept'
            if (reply) context.onReplyUsed?.(direct.id)
          }
          break
        }
        case 'join':
          await chat.join.mutateAsync({
            name: first,
            key: others[0],
            networkId,
            guardSelection: true,
          })
          if (!currentIdentity()) return 'kept'
          break
        case 'part':
          await chat.leave.mutateAsync({ c: c!, reason: rest || undefined })
          break
        case 'nick': {
          const error = nicknameError(rest, nicknameLimit(network?.isupport), t)
          if (error) return (chat.notify(error), 'kept')
          return raw(`NICK ${rest}`) ? 'handled' : 'kept'
        }
        case 'topic':
          if (!raw(rest ? `TOPIC ${channel} :${markdownToIrc(rest)}` : `TOPIC ${channel}`))
            return 'kept'
          if (!rest && network) chat.activate(serverViewId(network.id))
          break
        case 'away':
          return raw(rest ? `AWAY :${rest}` : 'AWAY :Away') ? 'handled' : 'kept'
        case 'status':
          return await runStatusCommand(rest, network?.id, {
            edit: (id) => actions.status(id),
            update: (id, input) => chat.networkProfile.mutateAsync({ id, input }),
            notify: (key) => chat.notify(t(`workspaceUi.commands.status.${key}`)),
          })
        case 'afk':
        case 'brb':
          return raw(`AWAY :${briefAwayText(name, rest)}`) ? 'handled' : 'kept'
        case 'back':
          return raw('AWAY') ? 'handled' : 'kept'
        case 'op':
        case 'deop':
        case 'voice':
        case 'devoice': {
          const mode = { op: '+o', deop: '-o', voice: '+v', devoice: '-v' }[name]
          return raw(`MODE ${channel} ${mode} ${first}`) ? 'handled' : 'kept'
        }
        case 'kick':
          return raw(`KICK ${channel} ${first}${afterFirst ? ` :${afterFirst}` : ''}`)
            ? 'handled'
            : 'kept'
        case 'ban':
          return raw(`MODE ${channel} +b ${first.includes('!') ? first : `${first}!*@*`}`)
            ? 'handled'
            : 'kept'
        case 'invite':
          if (!others[0] && !channel) return (usage(), 'kept')
          return raw(`INVITE ${first} ${others[0] ?? channel}`) ? 'handled' : 'kept'
        case 'mode':
          return raw(/^[+-]/.test(first) && channel ? `MODE ${channel} ${rest}` : `MODE ${rest}`)
            ? 'handled'
            : 'kept'
        case 'whois':
          if (!raw(`WHOIS ${first}`)) return 'kept'
          if (network) chat.activate(serverViewId(network.id))
          break
        case 'ignore':
        case 'unignore': {
          const notifications = useNotificationStore()
          if (!notifications.loaded) {
            chat.notify(t('workspaceUi.commands.settingsLoading'))
            return 'kept'
          }
          const identity = (m?: { nick: string; username?: string; host?: string }) =>
            m && { nick: m.nick, user: m.username, host: m.host }
          const out = ignoreCommand(name, rest, notifications.settings.ignore, {
            person: identity(chat.memberByNick(rest.replace(/^@/, ''))),
            self: identity(chat.member('you')),
            translate: (key, values, plural) =>
              values ? (plural === undefined ? t(key, values) : t(key, values, plural)) : t(key),
          })
          if (out.ignore && !notifications.update({ ignore: out.ignore })) return 'kept'
          chat.notify(out.notice)
          break
        }
        case 'list':
          actions.join(rest)
          break
        case 'server':
          if (network) chat.activate(serverViewId(network.id))
          break
        case 'settings':
          actions.channelSettings(c!.id)
          break
        case 'banlist':
          actions.channelSettings(c!.id, 'list:b')
          break
        case 'quote':
          return raw(rest) ? 'handled' : 'kept'
        case 'attract':
          if (!network?.features.attract)
            return (chat.notify(t('workspaceUi.commands.attractUnsupported')), 'kept')
          else await chat.attract(c!.id)
          break
        case 'gif': {
          // Bound to the conversation it was typed in, whatever is open when the search returns.
          const target = { conversationId: chat.activeId, replyTo: context.replyTo }
          if ((await gif.run(rest, target)) !== 'sent') return 'kept'
          if (target.replyTo) context.onReplyUsed?.(target.conversationId)
          return 'handled'
        }
      }
    } catch (error) {
      chat.notify(name === 'join' ? t('workspaceUi.commandEditor.failed') : errorText(error, t))
      return 'kept'
    }
    return 'handled'
  }

  /** The query of a `/gif` lookup running for this conversation, if any. */
  return { run, gifLookup: gif.lookup }
}
