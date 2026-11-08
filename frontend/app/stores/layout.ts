import { acceptHMRUpdate, defineStore } from 'pinia'
import { sanitizeCollapsed, sectionKey, toggleCollapsed } from '../../shared/utils/sidebar'

export const layoutLimits = {
  sidebar: { min: 200, max: 420, default: 248 },
  members: { min: 180, max: 380, default: 232 },
  pickerWidth: { min: 340, max: 820, default: 480 },
  pickerHeight: { min: 300, max: 760, default: 460 },
} as const
type Limit = { min: number; max: number; default: number }

const clamp = (value: unknown, limit: Limit) =>
  typeof value === 'number' && Number.isFinite(value)
    ? Math.round(Math.max(limit.min, Math.min(limit.max, value)))
    : limit.default

interface Layout {
  sidebarWidth: number
  membersWidth: number
  showMembers: boolean
  /** The network rail (server list) is docked; off collapses it where it would dock. */
  showRail: boolean
  /** The conversation list is docked; off collapses it where it would dock. */
  showSidebar: boolean
  /** Collapsed conversation-list sections, as sectionKey(network, section). */
  collapsedSections: string[]
  /** Sidebar shows only conversations with unread messages. */
  unreadOnly: boolean
  pickerWidth: number
  pickerHeight: number
}
function sanitize(value: unknown): Layout {
  const v = (value && typeof value === 'object' ? value : {}) as Record<string, unknown>
  return {
    sidebarWidth: clamp(v.sidebarWidth, layoutLimits.sidebar),
    membersWidth: clamp(v.membersWidth, layoutLimits.members),
    showMembers: typeof v.showMembers === 'boolean' ? v.showMembers : true,
    showRail: typeof v.showRail === 'boolean' ? v.showRail : true,
    showSidebar: typeof v.showSidebar === 'boolean' ? v.showSidebar : true,
    collapsedSections: sanitizeCollapsed(v.collapsedSections),
    unreadOnly: v.unreadOnly === true,
    pickerWidth: clamp(v.pickerWidth, layoutLimits.pickerWidth),
    pickerHeight: clamp(v.pickerHeight, layoutLimits.pickerHeight),
  }
}

/** Panel sizes belong to this device (screens differ), so they are saved locally only. */
export const useLayoutStore = defineStore('layout', () => {
  const state = reactive(readLocal('ircord.layout', sanitize))
  let timer: ReturnType<typeof setTimeout> | undefined
  watch(state, () => {
    clearTimeout(timer)
    timer = setTimeout(() => writeLocal('ircord.layout', state), 150)
  })
  // Save immediately when the page goes away, so a resize just before reload is kept.
  window.addEventListener('pagehide', () => {
    clearTimeout(timer)
    writeLocal('ircord.layout', state)
  })
  // Another tab resized something: follow it.
  window.addEventListener('storage', (event) => {
    if (event.key !== 'ircord.layout') return
    try {
      Object.assign(state, sanitize(JSON.parse(event.newValue || '{}')))
    } catch {
      /* Ignore malformed values written by something else. */
    }
  })
  function set<K extends keyof Layout>(key: K, value: Layout[K]) {
    const limits: Partial<Record<keyof Layout, Limit>> = {
      sidebarWidth: layoutLimits.sidebar,
      membersWidth: layoutLimits.members,
      pickerWidth: layoutLimits.pickerWidth,
      pickerHeight: layoutLimits.pickerHeight,
    }
    const limit = limits[key]
    state[key] = (limit ? clamp(value, limit) : value) as Layout[K]
  }
  const collapsed = (networkId: string, sectionId: string) =>
    state.collapsedSections.includes(sectionKey(networkId, sectionId))
  function toggleSection(networkId: string, sectionId: string) {
    state.collapsedSections = toggleCollapsed(
      state.collapsedSections,
      sectionKey(networkId, sectionId),
    )
  }
  return { state, set, collapsed, toggleSection }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useLayoutStore, import.meta.hot))
