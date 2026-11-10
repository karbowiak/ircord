import { isEditable, overlayOpen } from '../../../shared/utils/focus'
import { nextNetwork, nextUnread } from '../../../shared/utils/navigation'
import {
  matchesChord,
  channelShortcuts,
  shortcutOf,
  type ShortcutName,
} from '../../../shared/utils/shortcuts'
import { isConversationPath } from '../../../shared/utils/workspace-routes'

/** Overlays a workspace shortcut can open, and so also close again. */
export type ShortcutOverlay = 'search' | 'settings' | 'shortcuts' | 'inbox' | 'saved' | 'join'

/** The workspace-wide actions; bold, italic and underline belong to the composer. */
const workspaceActions = [
  'search',
  'gifs',
  'settings',
  'help',
  'previous',
  'next',
  'previousUnread',
  'nextUnread',
  'previousNetwork',
  'nextNetwork',
  'jumpToUnread',
  'jumpToPresent',
  'markRead',
  'markNetworkRead',
  'focusComposer',
  'toggleMembers',
  'inbox',
  'saved',
  'join',
] as const satisfies readonly ShortcutName[]
/** Holding these keeps stepping, like holding an arrow key. */
const repeating: ShortcutName[] = ['previous', 'next', 'previousNetwork', 'nextNetwork']

/**
 * Workspace keyboard shortcuts; the actions and their default keys live in
 * shared/utils/shortcuts.ts, and your own bindings come from your preferences (useShortcuts). The
 * help list (ShortcutList.vue) reads the same table.
 *
 * Overlays never stack: a shortcut whose own overlay is open (and on top) closes it, and while
 * any other dialog, picker, menu or listbox is open the shortcuts do nothing and leave the key to
 * the browser and the focused control. The one exception is Ctrl/⌘ G, which switches an open
 * emoji picker to GIFs (ChatComposer owns that). Keys a focused control already handled (the
 * composer's formatting, Up to edit) are left alone. Key repeat never re-triggers an action,
 * except stepping through conversations and networks while the keys are held.
 */
export function useWorkspaceShortcuts(handlers: {
  escape: () => void
  /** The workspace overlay that is open, if any. */
  overlay: () => string | null
  /** Closes the open overlay the way its own close button would (restoring focus). */
  close: () => void
  toggleMembers: () => void
}) {
  const { t } = useI18n()
  const chat = useChatStore()
  const router = useRouter()
  const navigation = useConversationNavigation()
  const actions = useWorkspaceActions()
  const markRead = useMarkRead()
  const dialogs = useDialogStack()
  const channels = useChannelNavigation()
  const { bindings } = useShortcuts()

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
    const name = shortcutOf(event, workspaceActions, bindings.value)
    if (!name) {
      if (
        event.defaultPrevented ||
        event.repeat ||
        event.shiftKey ||
        handlers.overlay() ||
        overlayOpen()
      )
        return
      const index = channelShortcuts.findIndex((chord) => matchesChord(event, chord))
      const channel = channels.value[index]
      if (channel) {
        void navigation.open(channel.id)
        event.preventDefault()
      }
      return
    }
    const escape = name === 'markRead' || name === 'markNetworkRead'
    // Something focused already handled the key (formatting, a menu); Escape still marks read.
    if (event.defaultPrevented && !escape) return
    if (event.repeat && !repeating.includes(name)) {
      // Swallow repeats of a shortcut that already ran, so the browser doesn't act on them.
      if (!escape && handledFirst) event.preventDefault()
      return
    }
    handledFirst = run(name, event)
    if (handledFirst) event.preventDefault()
  }

  function run(name: (typeof workspaceActions)[number], event: KeyboardEvent): boolean {
    switch (name) {
      case 'search':
        if (overlayOpen()) return false
        actions.search()
        return true
      case 'settings':
        return toggle('settings', () => actions.settings())
      case 'inbox':
        return toggle('inbox', () => actions.inbox())
      case 'saved':
        return toggle('saved', () => actions.saved())
      case 'join':
        return toggle('join', () => actions.join())
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
          else chat.notify(t('workspaceUi.history.nothingUnread'))
        }
        return true
      }
      case 'previousNetwork':
      case 'nextNetwork': {
        if (overlayOpen()) return false
        const next = nextNetwork(
          chat.data?.networks ?? [],
          chat.networkId,
          name === 'nextNetwork' ? 1 : -1,
        )
        if (next) chat.selectNetwork(next.id)
        return true
      }
      case 'jumpToUnread':
        if (overlayOpen()) return false
        return actions.jumpToUnread()
      case 'jumpToPresent':
        if (overlayOpen()) return false
        return actions.jumpToPresent()
      case 'focusComposer':
        if (overlayOpen()) return false
        return actions.focusComposer()
      case 'toggleMembers':
        if (
          overlayOpen() ||
          chat.serverView ||
          chat.active?.kind !== 'channel' ||
          chat.active.party
        )
          return false
        handlers.toggleMembers()
        return true
      case 'markRead':
      case 'markNetworkRead':
        if (overlayOpen() || !isConversationPath(router.currentRoute.value.path)) return false
        handlers.escape()
        if (name === 'markNetworkRead') {
          if (chat.network) void markRead.network(chat.network.id)
        } else if (chat.active) void chat.markRead(chat.active.id)
        // Escape also closes things the browser knows about; leave its default alone.
        return event.key !== 'Escape'
    }
    return false
  }
  useEventListener('keydown', keydown)
}
