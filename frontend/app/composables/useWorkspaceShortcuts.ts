import { isEditable, overlayOpen } from '../../shared/utils/focus'
import { nextUnread } from '../../shared/utils/navigation'

/**
 * Workspace keyboard shortcuts (listed in ShortcutList.vue):
 * Ctrl/⌘ K search · Ctrl/⌘ , settings · ? or Ctrl/⌘ / shortcuts · Alt ↑/↓ previous/next
 * conversation · Alt Shift ↑/↓ next unread · Esc mark read (and close drawers, cancel reply) ·
 * Shift Esc mark the network read. Open dialogs, menus and pickers keep these keys.
 */
export function useWorkspaceShortcuts(handlers: { escape: () => void }) {
  const chat = useChatStore()
  const actions = useWorkspaceActions()
  const markRead = useMarkRead()

  function keydown(event: KeyboardEvent) {
    const mod = event.metaKey || event.ctrlKey
    if (mod && event.key.toLowerCase() === 'k') {
      event.preventDefault()
      actions.search()
    } else if (mod && event.key === ',') {
      event.preventDefault()
      actions.settings()
    } else if ((mod && event.key === '/') || (event.key === '?' && !isEditable(event.target))) {
      if (overlayOpen()) return
      event.preventDefault()
      actions.shortcuts()
    } else if (event.altKey && (event.key === 'ArrowDown' || event.key === 'ArrowUp')) {
      if (overlayOpen()) return
      event.preventDefault()
      const direction = event.key === 'ArrowDown' ? 1 : -1
      if (!event.shiftKey) return chat.step(direction)
      const next = nextUnread(chat.data?.conversations ?? [], chat.activeId, direction)
      if (next) chat.activate(next.id)
      else chat.notify('Nothing unread.')
    } else if (event.key === 'Escape' && !overlayOpen()) {
      handlers.escape()
      if (event.shiftKey) {
        if (chat.network) void markRead.network(chat.network.id)
      } else if (chat.active) void chat.markRead(chat.active.id)
    }
  }
  onMounted(() => window.addEventListener('keydown', keydown))
  onBeforeUnmount(() => window.removeEventListener('keydown', keydown))
}
