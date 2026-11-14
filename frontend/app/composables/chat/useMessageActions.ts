import type { Message } from '../../../shared/types/chat'
import type { EmojiItem } from '../../../shared/types/media'
import { emojiText } from '../../../shared/utils/media'
import { stripFormatting } from '../../../shared/utils/irc'

/** Shared message eligibility and actions for menus, the toolbar, and the composer. */
export function useMessageActions() {
  const { t, n } = useI18n()
  const chat = useChatStore()
  const library = useLibraryStore()
  const { confirm } = usePrompt()
  const moderation = useModeration()

  /** How you would delete someone else's message: as a channel operator or server owner. */
  const moderatorOf = (m: Message) =>
    m.senderId === 'you'
      ? undefined
      : chat.moderationAuthorityIn(chat.data?.conversations.find((c) => c.id === m.conversationId))

  /**
   * Whether this confirmed message can be redacted on its network: your own, or anyone's where
   * you are a channel operator or a server owner with the server's moderation login.
   */
  function deletion(m: Message): { allowed: boolean; hint?: string; why?: string } | undefined {
    if (
      (m.senderId !== 'you' && !moderatorOf(m)) ||
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

  /** Whether this own confirmed text message can be edited on its network. */
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

  /**
   * Deletes someone else's message as a moderator. A server owner can add a reason and also
   * delete the files this server hosts for it; both are recorded in the audit log.
   */
  async function moderateMessage(m: Message, authority: 'operator' | 'owner') {
    const files = !!(m.attachment || m.attachments?.length)
    const choice = await moderation.confirmMessage({
      message: m,
      nick: chat.member(m.senderId)?.nick ?? m.senderId,
      owner: authority === 'owner',
      files,
    })
    if (!choice) return
    const result = await chat.redact(m, choice)
    if (!result) return
    if (result.filesDeleted)
      return chat.notify(
        result.attachmentRetained
          ? t('workspaceUi.menus.someFilesKept')
          : t('workspaceUi.moderation.messageFilesDeleted'),
      )
    chat.notify(
      result.attachmentRetained
        ? t('workspaceUi.moderation.othersFilesKept')
        : t('workspaceUi.menus.deleted'),
    )
  }

  /** Asks, deletes for everyone, then offers to remove an uploaded file the message carried. */
  async function deleteMessage(m: Message) {
    const authority = moderatorOf(m)
    if (authority) return moderateMessage(m, authority)
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

  return { deletion, editing, deleteMessage, reactWith, unreadable, copyMessage }
}
