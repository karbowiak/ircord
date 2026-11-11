import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import { linksClassificationOverrides, linkSourceKey } from '../../queries/links'

/** Resolve a message's own channel and server; never borrow the selected server. */
export function useMediaSensitivity(conversationId: MaybeRefOrGetter<string | undefined>) {
  const chat = useChatStore()
  const preferences = usePreferencesStore()
  return computed<'nsfw' | 'unknown' | null>(() => {
    const id = toValue(conversationId)
    const conversation = id && chat.data?.conversations.find((item) => item.id === id)
    if (!conversation) return 'unknown'
    if (conversation.kind !== 'channel' || conversation.party) return null
    const networkId = conversation.networkId
    if (
      chat.data?.networks.some((network) => network.id === networkId) &&
      preferences.allowsNsfw(networkId)
    )
      return null
    const key = linkSourceKey(networkId, conversation.id)
    const current = conversation.channelSettings?.nsfw
    const classification =
      typeof current === 'boolean' ? current : linksClassificationOverrides.get(key)
    return classification === false ? null : classification === true ? 'nsfw' : 'unknown'
  })
}
