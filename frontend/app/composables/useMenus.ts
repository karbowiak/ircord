import type { Conversation, Member, Message, Network } from '../../shared/types/chat'
import type { EmojiItem } from '../../shared/types/media'
import { emojiText } from '../../shared/utils/media'
import type { NotifyLevel } from '../../shared/notifications'
import { channelRights, channelTarget, topPrefix, stripFormatting } from '../../shared/utils/irc'
import { serverViewId } from '../stores/chat'
import { savable } from '../stores/saved'
import { isSortableSection } from '../../shared/utils/sidebar-order'
import type { MenuEntry } from './useContextMenu'

/** Right-click menus for people, conversations, networks and messages. */
export function useMenus() {
  const { t, n, locale } = useI18n()
  const chat = useChatStore()
  const library = useLibraryStore()
  const saved = useSavedStore()
  const notifications = useNotificationStore()
  const actions = useWorkspaceActions()
  const messageEditing = useMessageEditing()
  const markRead = useMarkRead()
  const order = useSidebarOrder()
  const { open, close, state } = useContextMenu()
  watch(locale, () => {
    if (state.open) close(false)
  })
  const { ask, confirm } = usePrompt()

  const copy = async (text: string, key: string) => {
    try {
      await navigator.clipboard.writeText(text)
      chat.notify(t(key))
    } catch {
      chat.notify(t('workspaceUi.controls.copyFailed'))
    }
  }
  const mention = (nick: string) => (chat.pendingInsert = `@${nick} `)
  const channelName = channelTarget

  const levelText = (level: NotifyLevel) =>
    t(
      {
        all: 'workspaceUi.menus.levelAll',
        mentions: 'workspaceUi.menus.levelMentions',
        none: 'workspaceUi.menus.levelNone',
      }[level],
    )
  /** The keyboard way to reorder the sidebar, next to dragging: one step, or back to A to Z. */
  function orderItems(c: Conversation): MenuEntry[] {
    const section = order.sectionOf(c)
    if (!isSortableSection(section)) return []
    const { up, down } = order.steps(c)
    return [
      ...(up
        ? [
            {
              label: t('workspaceUi.menus.moveUp'),
              icon: 'moveUp',
              action: () => order.step(c, -1),
            },
          ]
        : []),
      ...(down
        ? [
            {
              label: t('workspaceUi.menus.moveDown'),
              icon: 'moveDown',
              action: () => order.step(c, 1),
            },
          ]
        : []),
      ...(order.custom(c.networkId, section)
        ? [
            {
              label: t(
                section === 'favorites'
                  ? 'workspaceUi.menus.sortFavorites'
                  : 'workspaceUi.menus.sortChannels',
              ),
              icon: 'sortAz',
              action: () => order.sortByName(c.networkId, section),
            },
          ]
        : []),
    ]
  }
  function notificationItems(c: Conversation): MenuEntry[] {
    if (c.kind === 'channel') return channelNotificationItems(c)
    const current = notifications.settings.levels[c.id]
    const choice = (value: NotifyLevel | null, label: string) => ({
      label,
      checked: (current ?? null) === value,
      action: () => notifications.setLevel(c, value),
    })
    return [
      choice(
        null,
        t('workspaceUi.menus.default', { level: levelText(notifications.settings.directDefault) }),
      ),
      '-' as const,
      choice('all', t('workspaceUi.menus.allMessages')),
      choice('mentions', t('workspaceUi.menus.onlyMentions')),
      choice('none', t('workspaceUi.menus.mute')),
    ]
  }
  /** A channel's level is channel-wide: operators pick it, everyone else sees it. */
  function channelNotificationItems(c: Conversation): MenuEntry[] {
    const current = c.channelSettings?.notifications
    const editable = !!current && channelRights('you', c, chat.networkOf(c)).op
    const choice = (value: 'default' | NotifyLevel, label: string) => ({
      label,
      checked: current === value,
      disabled: !editable,
      action: () => {
        if (current !== value) chat.channelSettings.mutate({ c, changes: { notifications: value } })
      },
    })
    return [
      {
        label: t(editable ? 'workspaceUi.menus.everyone' : 'workspaceUi.menus.operatorsOnly'),
        disabled: true,
      },
      '-' as const,
      choice(
        'default',
        t('workspaceUi.menus.channelDefault', {
          level: levelText(notifications.settings.channelDefault),
        }),
      ),
      choice('all', t('workspaceUi.menus.allMessages')),
      choice('mentions', t('workspaceUi.menus.onlyMentions')),
      choice('none', t('workspaceUi.menus.mute')),
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
                        label: t('workspaceUi.menus.removeOperator'),
                        icon: 'shield',
                        action: () => send(`MODE ${target} -o ${person.nick}`),
                      }
                    : {
                        label: t('workspaceUi.menus.makeOperator'),
                        icon: 'shield',
                        action: () => send(`MODE ${target} +o ${person.nick}`),
                      },
                ]
              : []),
            ...(rights.voice
              ? [
                  theirs.includes('+')
                    ? {
                        label: t('workspaceUi.menus.removeVoice'),
                        icon: 'volume',
                        action: () => send(`MODE ${target} -v ${person.nick}`),
                      }
                    : {
                        label: t('workspaceUi.menus.giveVoice'),
                        icon: 'volume',
                        action: () => send(`MODE ${target} +v ${person.nick}`),
                      },
                ]
              : []),
            ...(rights.kick
              ? [
                  '-' as const,
                  {
                    label: t('workspaceUi.menus.kickAction'),
                    icon: 'logout',
                    danger: true,
                    action: async () => {
                      const reason = await ask({
                        title: t('workspaceUi.menus.kickTitle', {
                          nick: person.nick,
                          channel: target,
                        }),
                        label: t('workspaceUi.menus.reasonOptional'),
                        allowEmpty: true,
                        confirm: t('workspaceUi.menus.kick'),
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
                    label: t('workspaceUi.menus.ban'),
                    icon: 'lock',
                    danger: true,
                    action: async () => {
                      if (
                        await confirm({
                          title: t('workspaceUi.menus.banTitle', {
                            nick: person.nick,
                            channel: target,
                          }),
                          body: t('workspaceUi.menus.banBody', { nick: person.nick }),
                          confirm: t('workspaceUi.menus.ban'),
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
        {
          label: t('workspaceUi.menus.profile'),
          icon: 'user',
          action: () => actions.profile(person.id),
        },
        ...(!self
          ? [
              {
                label: t('workspaceUi.menus.message'),
                icon: 'message',
                action: () => void chat.direct.mutateAsync(person.id).catch(() => {}),
              },
              {
                label: t('workspaceUi.menus.mention'),
                icon: 'at',
                action: () => mention(person.nick),
              },
            ]
          : []),
        {
          label: t('workspaceUi.menus.copyNick'),
          icon: 'copy',
          action: () => copy(person.nick, 'workspaceUi.menus.copiedNick'),
        },
        ...(!self
          ? [
              {
                label: t(
                  notifications.ignored(person.nick)
                    ? 'workspaceUi.menus.stopIgnoring'
                    : 'workspaceUi.menus.ignore',
                ),
                icon: 'muted',
                action: () => {
                  const was = notifications.ignored(person.nick)
                  if (!notifications.toggleIgnore(person.nick)) return
                  chat.notify(
                    t(was ? 'workspaceUi.menus.showingAgain' : 'workspaceUi.menus.ignoring', {
                      nick: person.nick,
                    }),
                  )
                },
              },
            ]
          : []),
        {
          label: 'WHOIS',
          icon: 'info',
          hint: t('workspaceUi.menus.serverHint'),
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
          ? [
              {
                label: t('workspaceUi.menus.markRead'),
                icon: 'check',
                action: () => chat.markRead(c.id),
              },
            ]
          : []),
        {
          label: t(
            c.favorite ? 'workspaceUi.header.removeFavorite' : 'workspaceUi.header.addFavorite',
          ),
          icon: 'star',
          action: () => chat.favorite.mutate(c),
        },
        {
          label: t('workspaceUi.menus.notifications'),
          icon: 'bell',
          children: notificationItems(c),
        },
        ...orderItems(c),
        '-' as const,
        ...(c.kind === 'channel'
          ? [
              {
                label: t('workspaceUi.menus.channelSettings'),
                icon: 'settings',
                action: () => actions.channelSettings(c.id),
              },
              ...(rights.ban
                ? [
                    {
                      label: t('workspaceUi.menus.bans'),
                      icon: 'gavel',
                      action: () => actions.channelSettings(c.id, 'list:b'),
                    },
                  ]
                : []),
              ...(rights.topic
                ? [
                    {
                      label: t('workspaceUi.menus.editTopic'),
                      icon: 'type',
                      action: async () => {
                        const topic = await ask({
                          title: t('workspaceUi.menus.topicTitle', { channel: channelName(c) }),
                          label: t('workspaceUi.menus.topic'),
                          value: stripFormatting(c.topic),
                          confirm: t('workspaceUi.menus.setTopic'),
                        })
                        if (topic !== null)
                          chat.raw(`TOPIC ${channelName(c)} :${topic}`, c.networkId)
                      },
                    },
                  ]
                : []),
              {
                label: t('workspaceUi.menus.inviteAction'),
                icon: 'plus',
                action: async () => {
                  const nick = await ask({
                    title: t('workspaceUi.menus.inviteTitle', { channel: channelName(c) }),
                    label: t('workspaceUi.menus.nickname'),
                    confirm: t('workspaceUi.menus.invite'),
                  })
                  if (nick) chat.raw(`INVITE ${nick} ${channelName(c)}`, c.networkId)
                },
              },
              {
                label: t('workspaceUi.menus.copyChannel'),
                icon: 'copy',
                action: () => copy(channelName(c), 'workspaceUi.menus.copiedChannel'),
              },
              ...historyItems(c),
              '-' as const,
              {
                label: t('workspaceUi.menus.leaveChannel'),
                icon: 'logout',
                danger: true,
                action: () => chat.leave.mutate({ c }),
              },
            ]
          : [
              {
                label: t('workspaceUi.menus.profile'),
                icon: 'user',
                action: () => actions.profile(c.memberIds.find((id) => id !== 'you') ?? ''),
              },
              ...historyItems(c),
              '-' as const,
              {
                label: t('workspaceUi.menus.closeConversation'),
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
      {
        label: t('workspaceUi.menus.jumpDate'),
        icon: 'clock',
        action: () => actions.jumpToDate(c.id),
      },
      {
        label: t('workspaceUi.menus.exportLog'),
        icon: 'download',
        action: () => actions.exportLog(c.id),
      },
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
                label: t('workspaceUi.menus.markAllRead'),
                icon: 'check',
                hint: String(unread),
                action: () => void markRead.network(n.id),
              },
            ]
          : []),
        {
          label: t('workspaceUi.menus.serverWindow'),
          icon: 'terminal',
          action: () => chat.activate(serverViewId(n.id)),
        },
        {
          label: t('workspaceUi.menus.joinChannel'),
          icon: 'hash',
          action: () => {
            chat.selectNetwork(n.id)
            actions.join()
          },
        },
        {
          label: t('workspaceUi.menus.editNetwork'),
          icon: 'settings',
          action: () => actions.editNetwork(n.id),
        },
        '-' as const,
        {
          label: connected
            ? t('workspaceUi.menus.disconnect')
            : n.status === 'connecting'
              ? t('workspaceUi.menus.cancelConnection')
              : t('workspaceUi.menus.connect'),
          icon: connected ? 'offline' : 'refresh',
          action: () => chat.connection.mutate(n),
        },
        {
          label: t('workspaceUi.menus.copyAddress'),
          icon: 'copy',
          action: () => copy(`${n.host}:${n.port}`, 'workspaceUi.menus.copiedAddress'),
        },
        '-' as const,
        {
          label: t('workspaceUi.menus.removeNetwork'),
          icon: 'close',
          danger: true,
          action: async () => {
            if (
              await confirm({
                title: t('workspaceUi.menus.removeNetworkTitle', { network: n.name }),
                body: t('workspaceUi.menus.removeNetworkBody'),
                confirm: t('common.remove'),
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
   * messages, and only where the network would accept the redaction.
   */
  function deletion(m: Message): { allowed: boolean; hint?: string; why?: string } | undefined {
    if (
      m.senderId !== 'you' ||
      m.deleted ||
      m.delivery ||
      m.kind === 'event' ||
      m.kind === 'system' ||
      !chat.messageActionSupported(m, 'redact')
    )
      return undefined
    const why = chat.messageActionReason(m, 'redact')
    if (!why) return { allowed: true }
    return { allowed: false, hint: reasonHint(chat.messageActionReasonKey(m, 'redact')), why }
  }
  /**
   * Whether this message can be edited: only your own confirmed text messages, offered
   * disabled with the reason when the network or your login rules it out.
   */
  function editing(m: Message): { allowed: boolean; hint?: string; why?: string } | undefined {
    if (
      m.senderId !== 'you' ||
      m.deleted ||
      m.delivery ||
      (m.kind !== 'message' && m.kind !== 'action') ||
      m.gif ||
      m.attachment ||
      m.attachments?.length ||
      !chat.messageActionSupported(m, 'edit')
    )
      return undefined
    const why = chat.messageActionReason(m, 'edit')
    if (!why) return { allowed: true }
    return { allowed: false, hint: reasonHint(chat.messageActionReasonKey(m, 'edit')), why }
  }
  function reasonHint(key: string | undefined) {
    if (
      key === 'collectionsUi.features.edit.unsupported' ||
      key === 'collectionsUi.features.redact.unsupported'
    )
      return t('workspaceUi.menus.notSupported')
    if (
      key === 'collectionsUi.features.edit.accountRequired' ||
      key === 'collectionsUi.features.redact.accountRequired'
    )
      return t('workspaceUi.menus.needsAccount')
    if (
      key === 'collectionsUi.features.edit.otherAccount' ||
      key === 'collectionsUi.features.redact.otherAccount'
    )
      return t('workspaceUi.menus.otherAccount')
    return t('workspaceUi.menus.unavailable')
  }
  /** Asks, deletes for everyone, then offers to remove an uploaded file the message carried. */
  async function deleteMessage(m: Message) {
    if (
      !(await confirm({
        title: t('workspaceUi.menus.deleteTitle'),
        body: m.attachments?.length
          ? t('workspaceUi.menus.deleteAttachmentsBody')
          : m.attachment
            ? t('workspaceUi.menus.deleteAttachmentBody')
            : t('workspaceUi.menus.deleteBody'),
        confirm: t('workspaceUi.message.delete'),
        danger: true,
      }))
    )
      return
    const result = await chat.redact(m)
    if (!result) return
    if (!result.attachmentRetained) return chat.notify(t('workspaceUi.menus.deleted'))
    if (m.attachments?.length) {
      const ids = [...new Set(result.uploadIds ?? [])]
      if (!ids.length) return chat.notify(t('workspaceUi.menus.filesKept'))
      if (
        !(await confirm({
          title: t('workspaceUi.menus.deleteFilesTitle'),
          body: t('workspaceUi.menus.deleteFilesBody', { count: n(ids.length) }, ids.length),
          confirm: t('workspaceUi.menus.deleteFiles'),
          danger: true,
        }))
      )
        return chat.notify(t('workspaceUi.menus.filesKept'))
      let removed = 0
      for (const id of ids) if (await chat.deleteUpload(id)) removed++
      chat.notify(
        t(
          removed === m.attachments.length
            ? 'workspaceUi.menus.messageFilesDeleted'
            : removed
              ? 'workspaceUi.menus.someFilesKept'
              : 'workspaceUi.menus.filesKept',
        ),
      )
      return
    }
    if (!result.uploadId) return chat.notify(t('workspaceUi.menus.externalFileKept'))
    if (
      await confirm({
        title: t('workspaceUi.menus.deleteFileTitle'),
        body: t('workspaceUi.menus.deleteFileBody', {
          file: m.attachment?.name ?? t('workspaceUi.menus.uploadedFile'),
        }),
        confirm: t('workspaceUi.menus.deleteFile'),
        danger: true,
      })
    ) {
      if (await chat.deleteUpload(result.uploadId))
        chat.notify(t('workspaceUi.menus.messageFileDeleted'))
    } else chat.notify(t('workspaceUi.menus.fileKept'))
  }

  /** Reacts with a picked or quick emoji and records it as recently used. */
  function reactWith(m: Message, item: EmojiItem) {
    library.useEmoji(item)
    return chat.react(m, emojiText(item, library.library.skinTone))
  }
  /** Whether a message can start the unread boundary: delivered, real chat, not deleted. */
  const unreadable = (m: Message) =>
    !m.delivery &&
    !m.deleted &&
    (m.kind === 'message' || m.kind === 'action' || m.kind === 'notice')
  /** Copies the visible text, or explains how to copy it when the clipboard is unavailable. */
  async function copyMessage(m: Message) {
    try {
      await navigator.clipboard.writeText(stripFormatting(m.text))
      chat.notify(t('workspaceUi.menus.copiedClipboard'))
    } catch {
      chat.notify(t('workspaceUi.menus.selectToCopy'))
    }
  }

  /** `react` opens the full reaction picker for this message. */
  function message(event: MouseEvent | HTMLElement, m: Message, react?: () => void) {
    if (m.delivery || m.kind === 'event') return
    library.loadQuickReactions()
    const author = chat.member(m.senderId)
    const profile: MenuEntry[] = author
      ? [
          '-' as const,
          {
            label: t('workspaceUi.menus.profile'),
            icon: 'user',
            action: () => actions.profile(author.id),
          },
        ]
      : []
    const bookmark: MenuEntry[] =
      savable(m) || saved.isSaved(m.id)
        ? [
            {
              label:
                saved.pendingChange(m.id) === 'saving'
                  ? t('common.saving')
                  : saved.pendingChange(m.id) === 'removing'
                    ? t('workspaceUi.message.removingSaved')
                    : saved.isSaved(m.id)
                      ? t('workspaceUi.message.removeSaved')
                      : t('workspaceUi.message.saveLater'),
              icon: saved.isSaved(m.id) ? 'bookmarkSaved' : 'bookmark',
              disabled: saved.isPending(m.id),
              action: () => void saved.toggle(m),
            },
          ]
        : []
    if (m.deleted)
      return open(event, [...bookmark, ...profile].slice(bookmark.length ? 0 : 1), author?.nick)
    const removal = deletion(m)
    const edit = editing(m)
    const reactionReason = chat.messageActionReason(m, 'reactions')
    const replyReason = chat.messageActionReason(m, 'replies')
    open(
      event,
      [
        ...(react && chat.messageActionSupported(m, 'reactions')
          ? [
              {
                label: t('workspaceUi.menus.addReaction'),
                icon: 'smilePlus',
                disabled: !!reactionReason,
                hint: reactionReason ? t('workspaceUi.menus.unavailable') : undefined,
                title: reactionReason,
                action: react,
              },
            ]
          : []),
        ...(chat.messageActionSupported(m, 'replies')
          ? [
              {
                label: t('workspaceUi.message.reply'),
                icon: 'reply',
                disabled: !!replyReason,
                hint: replyReason ? t('workspaceUi.menus.unavailable') : undefined,
                title: replyReason,
                action: () => {
                  if (!chat.messageActionReason(m, 'replies')) actions.reply(m)
                },
              },
            ]
          : []),
        ...(edit
          ? [
              {
                label: t('workspaceUi.message.edit'),
                icon: 'pencil',
                disabled: !edit.allowed,
                hint: edit.hint,
                title: edit.why,
                action: () => {
                  if (!chat.messageActionReason(m, 'edit')) messageEditing.start(m)
                },
              },
            ]
          : []),
        ...(author && author.id !== 'you'
          ? [
              {
                label: t('workspaceUi.menus.mentionPerson', { nick: author.nick }),
                icon: 'at',
                action: () => mention(author.nick),
              },
            ]
          : []),
        {
          label: t('workspaceUi.message.copyText'),
          icon: 'copy',
          action: () => void copyMessage(m),
        },
        ...bookmark,
        ...(unreadable(m)
          ? [
              {
                label: t('workspaceUi.message.markUnread'),
                icon: 'unread',
                action: () => void chat.markUnread(m),
              },
            ]
          : []),
        ...profile,
        ...(removal
          ? [
              '-' as const,
              {
                label: t('workspaceUi.menus.deleteMessage'),
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
      (reactionReason || !chat.messageActionSupported(m, 'reactions')
        ? []
        : library.quickReactions
      ).map((item) => ({
        emoji: emojiText(item, library.library.skinTone),
        item,
        label: t('workspaceUi.message.reactWith', {
          emoji: item.unicode ? emojiText(item, library.library.skinTone) : `:${item.shortcode}:`,
        }),
        action: () => void reactWith(m, item),
      })),
    )
  }

  return {
    member,
    conversation,
    network,
    message,
    deletion,
    editing,
    deleteMessage,
    reactWith,
    unreadable,
    copyMessage,
  }
}
