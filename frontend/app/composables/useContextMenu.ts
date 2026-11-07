export interface MenuItem {
  label: string
  icon?: string
  hint?: string
  danger?: boolean
  disabled?: boolean
  /** Shows a check mark (radio-style choices such as notification levels). */
  checked?: boolean
  action?: () => void
  children?: MenuEntry[]
}
/** `'-'` draws a separator. */
export type MenuEntry = MenuItem | '-'
/** A row of emoji buttons above the items (quick reactions). */
export interface QuickAction {
  emoji: string
  label: string
  action: () => void
}

interface MenuState {
  open: boolean
  x: number
  y: number
  title: string
  items: MenuEntry[]
  quick: QuickAction[]
  /** Element to return focus to when the menu closes. */
  returnFocus: HTMLElement | null
}
const state = reactive<MenuState>({
  open: false,
  x: 0,
  y: 0,
  title: '',
  items: [],
  quick: [],
  returnFocus: null,
})

/**
 * One app-wide context menu. Open it from a `contextmenu` event, or from a button for dropdowns
 * (the menu opens below the button).
 */
export function useContextMenu() {
  function open(
    event: MouseEvent | HTMLElement,
    items: MenuEntry[],
    title = '',
    quick: QuickAction[] = [],
  ) {
    const entries = items.filter(
      (item, i, all) => item !== '-' || (i > 0 && all[i - 1] !== '-' && i < all.length - 1),
    )
    if (!entries.length) return
    if (event instanceof MouseEvent) {
      event.preventDefault()
      // Keyboard-opened (Shift+F10 / menu key) events carry no pointer position.
      const target = event.currentTarget as HTMLElement | null
      const box = target?.getBoundingClientRect()
      state.x = event.clientX || (box ? box.left + 8 : 0)
      state.y = event.clientY || (box ? box.bottom : 0)
      state.returnFocus = target
    } else {
      const box = event.getBoundingClientRect()
      state.x = box.left
      state.y = box.bottom + 4
      state.returnFocus = event
    }
    state.items = entries
    state.title = title
    state.quick = quick
    state.open = true
  }
  function close(restoreFocus = true) {
    state.open = false
    if (restoreFocus) state.returnFocus?.focus()
  }
  return { state, open, close }
}
