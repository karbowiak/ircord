import { isEditable, overlayOpen } from '../../shared/utils/focus'
import { nextUnread } from '../../shared/utils/navigation'
import { shortcutOf } from '../../shared/utils/shortcuts'

/** Overlays a workspace shortcut can open, and so also close again. */
export type ShortcutOverlay = 'search' | 'settings' | 'shortcuts'

/**
 * Workspace keyboard shortcuts; the bindings live in shared/utils/shortcuts.ts, which the help
 * list (ShortcutList.vue) reads too.
 *
 * Overlays never stack: a shortcut whose own overlay is open (and on top) closes it, and while
 * any other dialog, picker, menu or listbox is open the shortcuts do nothing and leave the key to
 * the browser and the focused control. The one exception is Ctrl/⌘ G, which switches an open
 * emoji picker to GIFs (ChatComposer owns that). Key repeat never re-triggers an action, except
 * stepping through conversations while Alt and an arrow are held.
 */
export function useWorkspaceShortcuts(handlers: {
  escape: () => void
  /** The workspace overlay that is open, if any. */
  overlay: () => string | null
  /** Closes the open overlay the way its own close button would (restoring focus). */
  close: () => void
}) {
  const chat = useChatStore()
  const actions = useWorkspaceActions()
  const markRead = useMarkRead()
  const dialogs = useDialogStack()

  function toggle(overlay: ShortcutOverlay, open: () => void) {
    if (handlers.overlay() === overlay) {
      // Something (a confirm prompt) is open over it: that one keeps the keyboard.
      if (dialogs.count > 1) return false
      handlers.close()
    } else if (!overlayOpen()) open()
    else return false
    return true
  }

  /** Whether the last fresh (non-repeat) shortcut press was handled. */
  let handledFirst = false
  function keydown(event: KeyboardEvent) {
    if (event.isComposing || event.keyCode === 229) return
    const name = shortcutOf(event, [
      'search',
      'gifs',
      'settings',
      'help',
      'previous',
      'next',
      'previousUnread',
      'nextUnread',
      'markRead',
      'markNetworkRead',
    ])
    if (!name) return
    const escape = name === 'markRead' || name === 'markNetworkRead'
    // Something focused already handled the key (formatting, a menu); Escape still marks read.
    if (event.defaultPrevented && !escape) return
    if (event.repeat && !(name === 'previous' || name === 'next')) {
      // Swallow repeats of a shortcut that already ran, so the browser doesn't act on them.
      if (!escape && handledFirst) event.preventDefault()
      return
    }
    handledFirst = run(name, event)
    if (handledFirst) event.preventDefault()
  }

  function run(name: string, event: KeyboardEvent): boolean {
    switch (name) {
      case 'search':
        return toggle('search', () => actions.search())
      case 'settings':
        return toggle('settings', () => actions.settings())
      case 'help':
        // “?” is ordinary text in a field; Ctrl/⌘ / works everywhere.
        if (event.key === '?' && isEditable(event.target)) return false
        return toggle('shortcuts', () => actions.shortcuts())
      case 'gifs':
        return actions.gifs()
      case 'previous':
      case 'next':
      case 'previousUnread':
      case 'nextUnread': {
        if (overlayOpen()) return false
        const direction = name === 'next' || name === 'nextUnread' ? 1 : -1
        if (name === 'previous' || name === 'next') chat.step(direction)
        else {
          const next = nextUnread(chat.data?.conversations ?? [], chat.activeId, direction)
          if (next) chat.activate(next.id)
          else chat.notify('Nothing unread.')
        }
        return true
      }
      case 'markRead':
      case 'markNetworkRead':
        if (overlayOpen()) return false
        handlers.escape()
        if (name === 'markNetworkRead') {
          if (chat.network) void markRead.network(chat.network.id)
        } else if (chat.active) void chat.markRead(chat.active.id)
        // Escape also closes things the browser knows about; leave its default alone.
        return false
    }
    return false
  }
  onMounted(() => window.addEventListener('keydown', keydown))
  onBeforeUnmount(() => window.removeEventListener('keydown', keydown))
}
