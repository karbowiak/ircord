import { useQuery } from '@pinia/colada'
import type { MaybeRefOrGetter } from 'vue'
import { channelGuideQuery } from '../../queries/chat'
import { guideKey } from '../../../shared/utils/channel-guide'

/**
 * The channel guide for one network (#258) and the channels you're already in there. Joining
 * sends the chosen guide channels in one request and seeds the joined conversations.
 */
export function useChannelGuide(networkId: MaybeRefOrGetter<string>) {
  const chat = useChatStore()
  const guide = useQuery(() => channelGuideQuery(toValue(networkId)))
  const joined = computed(
    () =>
      new Set(
        chat.data?.conversations
          .filter((c) => c.networkId === toValue(networkId) && c.kind === 'channel' && !c.party)
          .map((c) => guideKey(c.target || `#${c.name}`)) ?? [],
      ),
  )
  const available = computed(
    () => !!guide.data.value?.available && guide.data.value.entries.length > 0,
  )
  const joining = ref(false)
  async function join(channels: string[]) {
    joining.value = true
    try {
      const result = await apiClient().joinFromGuide(toValue(networkId), channels)
      result.joined.forEach((c) => chat.rememberCreatedChannel(c))
      return result
    } finally {
      joining.value = false
    }
  }
  return { guide, joined, available, joining, join }
}
