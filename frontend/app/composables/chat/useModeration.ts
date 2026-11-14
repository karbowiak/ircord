import type { Conversation, Member, Message } from '../../../shared/types/chat'

/** What a moderator chose when deleting someone else's message. */
export interface MessageModerationChoice {
  reason: string
  deleteFiles: boolean
}
interface MessageModeration {
  kind: 'message'
  message: Message
  nick: string
  /** A server owner acting through the server's moderation login: reason and audit note. */
  owner: boolean
  /** The message carries files; a server owner may delete them too. */
  files: boolean
  resolve: (choice: MessageModerationChoice | null) => void
}
interface RecentModeration {
  kind: 'recent'
  conversation: Conversation
  member: Member
  owner: boolean
}
const current = shallowRef<MessageModeration | RecentModeration | null>(null)

/** Dialog state for moderator deletions, rendered by <ModerationDialog>. */
export function useModeration() {
  function close() {
    const request = current.value
    current.value = null
    if (request?.kind === 'message') request.resolve(null)
  }
  /** Asks to delete someone else's message; resolves null when cancelled. */
  const confirmMessage = (request: Omit<MessageModeration, 'kind' | 'resolve'>) =>
    new Promise<MessageModerationChoice | null>((resolve) => {
      close()
      current.value = { kind: 'message', ...request, resolve }
    })
  /** Opens "Delete recent messages" for one member of a channel. */
  function deleteRecent(conversation: Conversation, member: Member, owner: boolean) {
    close()
    current.value = { kind: 'recent', conversation, member, owner }
  }
  function settle(choice: MessageModerationChoice) {
    const request = current.value
    current.value = null
    if (request?.kind === 'message') request.resolve(choice)
  }
  return { current, confirmMessage, deleteRecent, close, settle }
}
