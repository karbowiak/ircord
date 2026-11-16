import type { Bootstrap, MessagePage } from '../../shared/types/chat'

export interface WorkspaceCheckpoint {
  version: 3
  accountId: string
  savedAt: string
  bootstrap: Bootstrap
  messages: Record<string, MessagePage>
}

export function readCheckpoint(accountId: string): WorkspaceCheckpoint | undefined {
  try {
    const value = JSON.parse(localStorage.getItem(`ircord.snapshot.${accountId}`) ?? 'null')
    if (
      value?.version !== 3 ||
      value.accountId !== accountId ||
      value.bootstrap?.demo ||
      value.bootstrap?.version !== 1 ||
      !Number.isSafeInteger(value.bootstrap.cursor) ||
      value.bootstrap.cursor < 0 ||
      !Array.isArray(value.bootstrap.networks) ||
      !Array.isArray(value.bootstrap.conversations) ||
      !Array.isArray(value.bootstrap.members) ||
      !value.bootstrap.user ||
      !value.messages ||
      typeof value.messages !== 'object' ||
      Array.isArray(value.messages) ||
      !Object.values(value.messages).every(
        (page: unknown) =>
          !!page &&
          typeof page === 'object' &&
          'items' in page &&
          Array.isArray(page.items) &&
          page.items.length <= 100 &&
          !('partial' in page && page.partial),
      )
    )
      return
    return value
  } catch {
    // Local storage can be unavailable or contain an older schema.
  }
}

/** Whether the checkpoint was stored; a full or blocked storage must not interrupt the workspace. */
export function writeCheckpoint(value: WorkspaceCheckpoint) {
  try {
    localStorage.setItem(`ircord.snapshot.${value.accountId}`, JSON.stringify(value))
    return true
  } catch {
    return false
  }
}
