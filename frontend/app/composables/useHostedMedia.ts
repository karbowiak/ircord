import type { Message } from '../../shared/types/chat'

/** Stored media never falls back to the original host while pending or failed. */
export function useHostedMedia(message: () => Pick<Message, 'media'>) {
  const chat = useChatStore()
  const enabled = computed(() => !!chat.data?.capabilities.mediaCDN)
  const records = computed(() => new Map(message().media?.map((item) => [item.source, item])))
  function source(url: string | undefined) {
    if (!url || !enabled.value) return url
    const item = records.value.get(url)
    return item?.status === 'ready' ? item.url : undefined
  }
  return { enabled, source, records }
}
