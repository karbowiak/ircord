import type { Conversation, Member, Message, Network } from '../../shared/types/chat'
import type { NotifyLevel } from '../../shared/notifications'
import { channelRights, channelTarget, topPrefix } from '../../shared/utils/irc'
import { stripFormatting } from '../../shared/utils/irc'
import { serverViewId } from '../stores/chat'
import type { MenuEntry } from './useContextMenu'

/** Right-click menus for people, conversations, networks and messages. */
export function useMenus() {
  const chat = useChatStore()
  const notifications = useNotificationStore()
  const actions = useWorkspaceActions()
  const markRead = useMarkRead()
  const { open } = useContextMenu()
  const { ask, confirm } = usePrompt()

  const copy = async (text: string, what: string) => {
    try {
      await navigator.clipboard.writeText(text)
      chat.notify(`Copied ${what}.`)
    } catch {
      chat.notify('Couldn’t copy to the clipboard.')
    }
  }
  const mention = (nick: string) => (chat.pendingInsert = `@${nick} `)
  const channelName = channelTarget

  function notificationItems(c: Conversation): MenuEntry[] {
    const current = notifications.settings.levels[c.id]
    const choice = (value: NotifyLevel | null, label: string) => ({
      label,
      checked: (current ?? null) === value,
      action: () => notifications.setLevel(c, value),
    })
    const fallback =
      c.kind === 'direct'
        ? notifications.settings.directDefault
        : notifications.settings.channelDefault
    return [
      choice(
        null,
        `Default (${{ all: 'all messages', mentions: 'mentions', none: 'nothing' }[fallback]})`,
      ),
      '-' as const,
      choice('all', 'All messages'),
      choice('mentions', 'Only mentions'),
      choice('none', 'Nothing (mute)'),
    ]
  }

  /** A person. In a channel, moderation actions appear when your modes allow them. */
  function member(event: MouseEvent | HTMLElement, person: Member, c = chat.active) {
    const network = c ? chat.networkOf(c) : chat.network
    const self = person.id === 'you'
    const rights = c?.kind === 'channel' ? channelRights('you', c, network) : undefined
    const theirs = c ? (c.memberModes?.[person.id] ?? '') : ''
    const send = (line: string) => chat.raw(line, network?.id)
    const target = c ? channelName(c) : ''
    const moderation: MenuEntry[] =
      rights && !self
        ? [
            ...(rights.op
              ? [
                  theirs.includes('@')
                    ? {
                        label: 'Remove operator',
                        icon: 'shield',
                        action: () => send(`MODE ${target} -o ${person.nick}`),
                      }
                    : {
                        label: 'Make operator',
                        icon: 'shield',
                        action: () => send(`MODE ${target} +o ${person.nick}`),
                      },
                ]
              : []),
            ...(rights.voice
              ? [
                  theirs.includes('+')
                    ? {
                        label: 'Remove voice',
                        icon: 'volume',
                        action: () => send(`MODE ${target} -v ${person.nick}`),
                      }
                    : {
                        label: 'Give voice',
                        icon: 'volume',
                        action: () => send(`MODE ${target} +v ${person.nick}`),
                      },
                ]
              : []),
            ...(rights.kick
              ? [
                  '-' as const,
                  {
                    label: 'Kick…',
                    icon: 'logout',
                    danger: true,
                    action: async () => {
                      const reason = await ask({
                        title: `Kick ${person.nick} from ${target}`,
                        label: 'Reason (optional)',
                        allowEmpty: true,
                        confirm: 'Kick',
                        danger: true,
                        requireText: undefined,
                      }).catch(() => null)
                      if (reason !== null)
                        send(`KICK ${target} ${person.nick}${reason ? ` :${reason}` : ''}`)
                    },
                  },
                ]
              : []),
            ...(rights.ban
              ? [
                  {
                    label: 'Ban',
                    icon: 'lock',
                    danger: true,
                    action: async () => {
                      if (
                        await confirm({
                          title: `Ban ${person.nick} from ${target}?`,
                          body: `Sets a ban on ${person.nick}!*@*. They can’t rejoin until it is removed.`,
                          confirm: 'Ban',
                          danger: true,
                        })
                      )
                        send(`MODE ${target} +b ${person.nick}!*@*`)
                    },
                  },
                ]
              : []),
          ]
        : []
    open(
      event,
      [
        { label: 'Profile', icon: 'user', action: () => actions.profile(person.id) },
        ...(!self
          ? [
              {
                label: 'Message',
                icon: 'message',
                action: () => void chat.direct.mutateAsync(person.id).catch(() => {}),
              },
              { label: 'Mention', icon: 'at', action: () => mention(person.nick) },
            ]
          : []),
        { label: 'Copy nick', icon: 'copy', action: () => copy(person.nick, 'nick') },
        ...(!self
          ? [
              {
                label: notifications.ignored(person.nick) ? 'Stop ignoring' : 'Ignore',
                icon: 'muted',
                action: () => {
                  const was = notifications.ignored(person.nick)
                  notifications.toggleIgnore(person.nick)
                  chat.notify(was ? `Showing ${person.nick} again.` : `Ignoring ${person.nick}.`)
                },
              },
            ]
          : []),
        {
          label: 'WHOIS',
          icon: 'info',
          hint: 'server window',
          action: () => {
            if (network && chat.raw(`WHOIS ${person.nick}`, network.id))
              chat.activate(serverViewId(network.id))
          },
        },
        ...(moderation.length ? ['-' as const, ...moderation] : []),
      ],
      `${topPrefix(person.id, c, network)}${person.nick}`,
    )
  }

  function conversation(event: MouseEvent | HTMLElement, c: Conversation) {
    const network = chat.networkOf(c)
    const rights = channelRights('you', c, network)
    const unread = c.unreadCount > 0 || c.mentionCount > 0
    open(
      event,
      [
        ...(unread
          ? [{ label: 'Mark as read', icon: 'check', action: () => chat.markRead(c.id) }]
          : []),
        {
          label: c.favorite ? 'Remove from favorites' : 'Add to favorites',
          icon: 'star',
          action: () => chat.favorite.mutate(c),
        },
        { label: 'Notifications', icon: 'bell', children: notificationItems(c) },
        '-' as const,
        ...(c.kind === 'channel'
          ? [
              {
                label: 'Channel settings…',
                icon: 'settings',
                action: () => actions.channelSettings(c.id),
              },
              ...(rights.ban
                ? [
                    {
                      label: 'Bans and lists…',
                      icon: 'gavel',
                      action: () => actions.channelSettings(c.id, 'list:b'),
                    },
                  ]
                : []),
              ...(rights.topic
                ? [
                    {
                      label: 'Edit topic…',
                      icon: 'type',
                      action: async () => {
                        const topic = await ask({
                          title: `Topic for ${channelName(c)}`,
                          label: 'Topic',
                          value: stripFormatting(c.topic),
                          confirm: 'Set topic',
                        })
                        if (topic !== null)
                          chat.raw(`TOPIC ${channelName(c)} :${topic}`, c.networkId)
                      },
                    },
                  ]
                : []),
              {
                label: 'Invite someone…',
                icon: 'plus',
                action: async () => {
                  const nick = await ask({
                    title: `Invite to ${channelName(c)}`,
                    label: 'Nickname',
                    confirm: 'Invite',
                  })
                  if (nick) chat.raw(`INVITE ${nick} ${channelName(c)}`, c.networkId)
                },
              },
              {
                label: 'Copy channel name',
                icon: 'copy',
                action: () => copy(channelName(c), 'channel name'),
              },
              ...historyItems(c),
              '-' as const,
              {
                label: 'Leave channel',
                icon: 'logout',
                danger: true,
                action: () => chat.leave.mutate({ c }),
              },
            ]
          : [
              {
                label: 'Profile',
                icon: 'user',
                action: () => actions.profile(c.memberIds.find((id) => id !== 'you') ?? ''),
              },
              ...historyItems(c),
              '-' as const,
              {
                label: 'Close conversation',
                icon: 'close',
                danger: true,
                action: () => chat.leave.mutate({ c }),
              },
            ]),
      ],
      c.kind === 'channel' ? channelName(c) : (chat.directPartner(c)?.nick ?? c.name),
    )
  }

  /** Log export and jumping back in time, for any conversation. */
  function historyItems(c: Conversation): MenuEntry[] {
    return [
      '-',
      { label: 'Jump to date…', icon: 'clock', action: () => actions.jumpToDate(c.id) },
      { label: 'Export log', icon: 'download', action: () => actions.exportLog(c.id) },
    ]
  }

  function network(event: MouseEvent | HTMLElement, n: Network) {
    const connected = n.status === 'connected'
    const unread = markRead.unread(n.id).length
    open(
      event,
      [
        ...(unread
          ? [
              {
                label: 'Mark all as read',
                icon: 'check',
                hint: String(unread),
                action: () => void markRead.network(n.id),
              },
            ]
          : []),
        {
          label: 'Server window',
          icon: 'terminal',
          action: () => chat.activate(serverViewId(n.id)),
        },
        {
          label: 'Join a channel…',
          icon: 'hash',
          action: () => {
            chat.selectNetwork(n.id)
            actions.join()
          },
        },
        { label: 'Edit network…', icon: 'settings', action: () => actions.editNetwork(n.id) },
        '-' as const,
        {
          label: connected
            ? 'Disconnect'
            : n.status === 'connecting'
              ? 'Cancel connection'
              : 'Connect',
          icon: connected ? 'offline' : 'refresh',
          action: () => chat.connection.mutate(n),
        },
        {
          label: 'Copy address',
          icon: 'copy',
          action: () => copy(`${n.host}:${n.port}`, 'address'),
        },
        '-' as const,
        {
          label: 'Remove network…',
          icon: 'close',
          danger: true,
          action: async () => {
            if (
              await confirm({
                title: `Remove ${n.name}?`,
                body: 'This disconnects and deletes its channels, history and saved login.',
                confirm: 'Remove',
                danger: true,
                requireText: n.name,
              })
            )
              chat.removeNetwork.mutate(n)
          },
        },
      ],
      n.name,
    )
  }

  /**
   * Whether this message can be deleted for everyone: only your own confirmed
   * messages, and only where the network negotiated message redaction.
   */
  function deletion(m: Message): { allowed: boolean; hint?: string; why?: string } | undefined {
    if (
      m.senderId !== 'you' ||
      m.deleted ||
      m.delivery ||
      m.kind === 'event' ||
      m.kind === 'system'
    )
      return undefined
    const why = chat.messageActionReason(m, 'redact')
    if (!why) return { allowed: true }
    const hint = why === 'This network does not support deleting messages' ? 'not supported' : 'unavailable'
    return { allowed: false, hint, why }
  }
  /** Asks, deletes for everyone, then offers to remove an uploaded file the message carried. */
  async function deleteMessage(m: Message) {
    if (
      !(await confirm({
        title: 'Delete message?',
        body: m.attachment
          ? 'Everyone in this conversation will see “Message deleted”. The uploaded file stays available at its link until you delete it.'
          : 'Everyone in this conversation will see “Message deleted”. Other clients may already have a copy.',
        confirm: 'Delete',
        danger: true,
      }))
    )
      return
    const result = await chat.redact(m)
    if (!result) return
    if (!result.attachmentRetained) return chat.notify('Message deleted.')
    if (!result.uploadId)
      return chat.notify(
        'Message deleted. Its attached file is hosted elsewhere and was not removed.',
      )
    if (
      await confirm({
        title: 'Delete the file too?',
        body: `The message is gone, but ${m.attachment?.name ?? 'the uploaded file'} is still available to anyone with its link.`,
        confirm: 'Delete file',
        danger: true,
      })
    ) {
      if (await chat.deleteUpload(result.uploadId)) chat.notify('Message and file deleted.')
    } else chat.notify('Message deleted. The file was kept.')
  }

  /** `react` opens the full reaction picker for this message. */
  function message(event: MouseEvent | HTMLElement, m: Message, react?: () => void) {
    if (m.delivery || m.kind === 'event') return
    const author = chat.member(m.senderId)
    const profile: MenuEntry[] = author
      ? ['-' as const, { label: 'Profile', icon: 'user', action: () => actions.profile(author.id) }]
      : []
    if (m.deleted) return open(event, profile.slice(1), author?.nick)
    const removal = deletion(m)
    const reactionReason = chat.messageActionReason(m, 'reactions')
    const replyReason = chat.messageActionReason(m, 'replies')
    open(
      event,
      [
        ...(react
          ? [
              {
                label: 'Add reaction…',
                icon: 'smilePlus',
                disabled: !!reactionReason,
                hint: reactionReason ? 'unavailable' : undefined,
                title: reactionReason,
                action: react,
              },
            ]
          : []),
        {
          label: 'Reply',
          icon: 'reply',
          disabled: !!replyReason,
          hint: replyReason ? 'unavailable' : undefined,
          title: replyReason,
          action: () => {
            if (!chat.messageActionReason(m, 'replies')) actions.reply(m)
          },
        },
        ...(author && author.id !== 'you'
          ? [{ label: `Mention ${author.nick}`, icon: 'at', action: () => mention(author.nick) }]
          : []),
        {
          label: 'Copy text',
          icon: 'copy',
          action: () => copy(stripFormatting(m.text), 'message'),
        },
        ...profile,
        ...(removal
          ? [
              '-' as const,
              {
                label: 'Delete message…',
                icon: 'trash',
                danger: true,
                disabled: !removal.allowed,
                hint: removal.hint,
                title: removal.why,
                action: () => void deleteMessage(m),
              },
            ]
          : []),
      ],
      author?.nick,
      (reactionReason ? [] : ['👍', '❤️', '😂', '😮', '😢']).map((emoji) => ({
        emoji,
        label: `React with ${emoji}`,
        action: () => void chat.react(m, emoji),
      })),
    )
  }

  return { member, conversation, network, message, deletion, deleteMessage }
}
