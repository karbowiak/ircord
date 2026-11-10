import type { Conversation } from '../../../shared/types/chat'
import { partyName } from '../../../shared/utils/party'
import { sectionKey } from '../../../shared/utils/sidebar'
import {
  isSortableSection,
  moveId,
  orderItems,
  stepId,
  type SortableSection,
} from '../../../shared/utils/sidebar-order'

/**
 * The conversation-list sections of a network in display order, and the actions that change
 * that order. Favorites and channels start A to Z; dragging or moving one keeps your order on
 * the account. Unread filters and collapsed sections never change this order.
 */
export function useSidebarOrder() {
  const chat = useChatStore()
  const preferences = usePreferencesStore()
  const auth = useAuthStore()
  const { t } = useI18n()
  const name = (c: Conversation) =>
    c.party
      ? partyName(c.party, auth.account?.id, t('workspaceUi.party.unnamed'))
      : c.kind === 'direct'
        ? (chat.directPartner(c)?.displayName ?? c.name)
        : c.name
  const byName = (a: Conversation, b: Conversation) =>
    name(a).localeCompare(name(b), undefined, { numeric: true, sensitivity: 'base' })

  /** Which section lists a conversation in the sidebar. */
  const sectionOf = (c: Conversation) =>
    c.favorite ? 'favorites' : c.kind === 'channel' && !c.party ? 'channels' : 'directs'
  const stored = (networkId: string, section: SortableSection) =>
    preferences.preferences.sidebarOrder[sectionKey(networkId, section)]
  const custom = (networkId: string, section: string) =>
    isSortableSection(section) && !!stored(networkId, section)

  /** Every conversation of a sortable section, in display order. */
  function items(networkId: string, section: SortableSection) {
    const members = (chat.data?.conversations ?? []).filter(
      (c) => c.networkId === networkId && sectionOf(c) === section,
    )
    return orderItems(members, stored(networkId, section), byName)
  }

  function save(networkId: string, section: SortableSection, ids: string[] | null) {
    const sidebarOrder = { ...preferences.preferences.sidebarOrder }
    if (ids) sidebarOrder[sectionKey(networkId, section)] = ids
    else delete sidebarOrder[sectionKey(networkId, section)]
    preferences.update({ sidebarOrder })
  }
  const ids = (networkId: string, section: SortableSection) =>
    items(networkId, section).map((c) => c.id)

  /** Drops `id` before or after `target` in the same section and keeps that order. */
  function move(
    networkId: string,
    section: SortableSection,
    id: string,
    target: string,
    place: 'before' | 'after',
  ) {
    const current = ids(networkId, section)
    const next = moveId(current, id, target, place)
    if (next.join('\n') !== current.join('\n')) save(networkId, section, next)
  }

  /** Where `c` can still move by one step: up, down, or both. */
  function steps(c: Conversation) {
    const section = sectionOf(c)
    if (!isSortableSection(section)) return { up: false, down: false }
    const list = ids(c.networkId, section)
    const index = list.indexOf(c.id)
    return { up: index > 0, down: index >= 0 && index < list.length - 1 }
  }
  function step(c: Conversation, direction: -1 | 1) {
    const section = sectionOf(c)
    if (!isSortableSection(section)) return
    const current = ids(c.networkId, section)
    const next = stepId(current, c.id, direction)
    if (next.join('\n') !== current.join('\n')) save(c.networkId, section, next)
  }

  /** Back to A to Z: forgets your order for that section. */
  function sortByName(networkId: string, section: SortableSection) {
    save(networkId, section, null)
  }

  return { name, sectionOf, custom, items, move, steps, step, sortByName }
}
