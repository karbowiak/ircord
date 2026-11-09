import type { FilterOption } from '../components/FilterMenu.vue'
import type { Conversation, Member } from '../../shared/types/chat'
import { searchHasKinds, type FilterChip } from '../../shared/utils/search-filter'

/** Names and choices for message search filters: people, conversations, kinds and dates. */
export function useSearchFilters() {
  const chat = useChatStore()
  const { day } = useTimeFormat()
  const networks = computed(() => chat.data?.networks ?? [])
  const networkName = (id: string) => networks.value.find((n) => n.id === id)?.name ?? ''
  const conversationLabel = (c: Conversation) =>
    c.kind === 'channel' ? `#${c.name}` : `@${chat.directPartner(c)?.nick ?? c.name}`
  const people = computed(() => (chat.data?.members ?? []).filter((m) => m.id !== 'you'))
  const memberName = (id: string) =>
    id === 'you' ? (chat.self?.displayName ?? 'You') : (chat.member(id)?.displayName ?? 'Someone')
  const dayLabel = (value: string) => day(new Date(`${value}T12:00:00`).toISOString())

  function chipLabel(chip: FilterChip) {
    switch (chip.key) {
      case 'from':
      case 'mentions':
        return memberName(chip.value)
      case 'in': {
        const c = chat.data?.conversations.find((x) => x.id === chip.value)
        return c ? conversationLabel(c) : 'a closed conversation'
      }
      case 'has':
        return searchHasKinds.find((k) => k.value === chip.value)?.label ?? chip.value
      case 'author':
        return chip.value === 'bot' ? 'bot' : 'person'
      default:
        return dayLabel(chip.value)
    }
  }

  const personOptions = computed<FilterOption[]>(() =>
    people.value.map((m: Member) => ({ value: m.id, label: `${m.displayName} (@${m.nick})` })),
  )
  const conversationOptions = computed<FilterOption[]>(() =>
    (chat.data?.conversations ?? []).map((c) => ({
      value: c.id,
      label: conversationLabel(c),
      group: networks.value.length > 1 ? networkName(c.networkId) : undefined,
    })),
  )
  const hasOptions: FilterOption[] = searchHasKinds.map((k) => ({ ...k }))
  return {
    people,
    memberName,
    conversationLabel,
    networkName,
    chipLabel,
    dayLabel,
    personOptions,
    conversationOptions,
    hasOptions,
  }
}
