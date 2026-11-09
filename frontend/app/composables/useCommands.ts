import type { Message } from '../../shared/types/chat'
import { nicknameError, nicknameLimit } from '../../shared/utils/nickname'
import { serverViewId } from '../stores/chat'
import { channelTarget, markdownToIrc, channelRights } from '../../shared/utils/irc'
import { createGifCommand } from '../../shared/utils/gif-command'
import { briefAwayText, slapAction, type CommandSpec } from '../../shared/utils/commands'
import { ignoreCommand } from '../../shared/utils/ignore-command'

export type { CommandSpec }

export const commands: CommandSpec[] = [
  { name: 'me', args: '<action>', description: 'Describe what you’re doing' },
  { name: 'slap', args: '<nick>', description: 'Slap someone around a bit with a large cucumber' },
  { name: 'msg', args: '<nick> <message>', description: 'Send someone a private message' },
  { name: 'query', args: '<nick>', description: 'Open a private conversation' },
  { name: 'join', args: '<#channel> [key]', description: 'Join a channel' },
  { name: 'part', args: '[reason]', description: 'Leave this channel', channel: true },
  { name: 'nick', args: '<new nick>', description: 'Change your nickname on this network' },
  {
    name: 'topic',
    args: '[new topic]',
    description: 'Show or change the channel topic',
    channel: true,
  },
  { name: 'away', args: '[message]', description: 'Mark yourself as away' },
  {
    name: 'afk',
    args: '[reason]',
    description: 'Mark yourself as away from keyboard until you next send a message',
  },
  {
    name: 'brb',
    args: '[reason]',
    description: 'Mark yourself as right back until you next send a message',
  },
  { name: 'back', args: '', description: 'Mark yourself as back' },
  { name: 'op', args: '<nick>', description: 'Make someone a channel operator', channel: true },
  { name: 'deop', args: '<nick>', description: 'Remove operator status', channel: true },
  { name: 'voice', args: '<nick>', description: 'Give someone voice', channel: true },
  { name: 'devoice', args: '<nick>', description: 'Remove voice', channel: true },
  {
    name: 'kick',
    args: '<nick> [reason]',
    description: 'Remove someone from the channel',
    channel: true,
  },
  { name: 'ban', args: '<nick>', description: 'Ban someone from the channel', channel: true },
  { name: 'invite', args: '<nick> [#channel]', description: 'Invite someone to a channel' },
  { name: 'mode', args: '<modes> [args]', description: 'Change channel or user modes' },
  { name: 'whois', args: '<nick>', description: 'Look someone up (shown in the server window)' },
  {
    name: 'ignore',
    args: '[nick]',
    description: 'Hide someone’s messages, or show who you ignore',
  },
  { name: 'unignore', args: '<nick>', description: 'Stop ignoring someone' },
  { name: 'list', args: '[filter]', description: 'Browse channels on this network' },
  { name: 'server', args: '', description: 'Open this network’s server window' },
  {
    name: 'settings',
    args: '',
    description: 'Channel settings: topic, modes and bans',
    channel: true,
  },
  { name: 'banlist', args: '', description: 'Show this channel’s bans', channel: true },
  { name: 'gif', args: '<search terms>', description: 'Send a random matching GIF' },
  { name: 'quote', args: '<raw IRC line>', description: 'Send a raw IRC command' },
  {
    name: 'attract',
    args: '',
    description: 'Make your last message sparkle for everyone here',
    channel: true,
    operator: true,
    hidden: true,
  },
]
const aliases: Record<string, string> = {
  raw: 'quote',
  j: 'join',
  leave: 'part',
  q: 'query',
  w: 'whois',
}

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
}

/** Runs slash commands typed in the composer. */
export function useCommands() {
  const { t } = useI18n()
  const chat = useChatStore()
  const media = useMediaStore()
  const actions = useWorkspaceActions()
  const gif = createGifCommand({
    search: async (query) => (await klipyGifs(media.appKey, query, 1)).items,
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
    const name = aliases[match[1]!.toLowerCase()] ?? match[1]!.toLowerCase()
    const rest = match[2]!.trim()
    const [first = '', ...others] = rest.split(/\s+/)
    const spec = commands.find((c) => c.name === name)
    const c = chat.active
    const network = chat.network
    const channel = c?.kind === 'channel' ? channelTarget(c) : ''
    const raw = (line: string) => chat.raw(line, network?.id)
    const usage = () =>
      chat.notify(t('workspaceUi.commands.usage', { command: spec!.name, args: spec!.args }))

    // Operator-only commands don't exist for anyone else, not even by name.
    if (!spec || (spec.operator && !channelRights('you', c, network).op)) {
      chat.notify(t('workspaceUi.commands.unknown', { command: match[1] }))
      return 'handled'
    }
    if (spec.channel && !channel) {
      chat.notify(t('workspaceUi.commands.channelOnly', { command: spec.name }))
      return 'handled'
    }
    const needs = spec.args.startsWith('<')
    if (needs && !rest) {
      usage()
      return 'handled'
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
          const text = others.join(' ')
          if (!person) {
            if (name === 'msg' && text) raw(`PRIVMSG ${first} :${text}`)
            else chat.notify(t('workspaceUi.commands.personMissing', { name: first }))
            break
          }
          const origin = chat.activeId
          const direct = await chat.direct.mutateAsync(person.id)
          if (name === 'msg' && text && !chat.send(text)) {
            if (chat.activeId === direct.id) chat.activate(origin)
            return 'kept'
          }
          break
        }
        case 'join':
          await chat.join.mutateAsync({ name: first, key: others[0] })
          break
        case 'part':
          await chat.leave.mutateAsync({ c: c!, reason: rest || undefined })
          break
        case 'nick': {
          const error = nicknameError(rest, nicknameLimit(network?.isupport), t)
          if (error) chat.notify(error)
          else raw(`NICK ${rest}`)
          break
        }
        case 'topic':
          raw(rest ? `TOPIC ${channel} :${markdownToIrc(rest)}` : `TOPIC ${channel}`)
          if (!rest && network) chat.activate(serverViewId(network.id))
          break
        case 'away':
          raw(rest ? `AWAY :${rest}` : 'AWAY :Away')
          break
        case 'afk':
        case 'brb':
          raw(`AWAY :${briefAwayText(name, rest)}`)
          break
        case 'back':
          raw('AWAY')
          break
        case 'op':
        case 'deop':
        case 'voice':
        case 'devoice': {
          const mode = { op: '+o', deop: '-o', voice: '+v', devoice: '-v' }[name]
          raw(`MODE ${channel} ${mode} ${first}`)
          break
        }
        case 'kick':
          raw(`KICK ${channel} ${first}${others.length ? ` :${others.join(' ')}` : ''}`)
          break
        case 'ban':
          raw(`MODE ${channel} +b ${first.includes('!') ? first : `${first}!*@*`}`)
          break
        case 'invite':
          if (!others[0] && !channel) return (usage(), 'handled')
          raw(`INVITE ${first} ${others[0] ?? channel}`)
          break
        case 'mode':
          raw(/^[+-]/.test(first) && channel ? `MODE ${channel} ${rest}` : `MODE ${rest}`)
          break
        case 'whois':
          if (raw(`WHOIS ${first}`) && network) chat.activate(serverViewId(network.id))
          break
        case 'ignore':
        case 'unignore': {
          const notifications = useNotificationStore()
          if (!notifications.loaded) {
            chat.notify(t('workspaceUi.commands.settingsLoading'))
            break
          }
          const identity = (m?: { nick: string; username?: string; host?: string }) =>
            m && { nick: m.nick, user: m.username, host: m.host }
          const out = ignoreCommand(name, rest, notifications.settings.ignore, {
            person: identity(chat.memberByNick(rest.replace(/^@/, ''))),
            self: identity(chat.member('you')),
            translate: (key, values, plural) =>
              values ? (plural === undefined ? t(key, values) : t(key, values, plural)) : t(key),
          })
          if (out.ignore && !notifications.update({ ignore: out.ignore })) break
          chat.notify(out.notice)
          break
        }
        case 'list':
          actions.join()
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
          raw(rest)
          break
        case 'attract':
          if (!network?.features.attract) chat.notify(t('workspaceUi.commands.attractUnsupported'))
          else await chat.attract(c!.id)
          break
        case 'gif': {
          // Bound to the conversation it was typed in, whatever is open when the search returns.
          const target = { conversationId: chat.activeId, replyTo: context.replyTo }
          return (await gif.run(rest, target)) === 'sent' ? 'handled' : 'kept'
        }
      }
    } catch (error) {
      chat.notify(errorText(error))
      return 'kept'
    }
    return 'handled'
  }

  /** The query of a `/gif` lookup running for this conversation, if any. */
  return { run, gifLookup: gif.lookup }
}
