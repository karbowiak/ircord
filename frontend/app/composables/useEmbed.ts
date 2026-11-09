import { computed, defineAsyncComponent, onBeforeUnmount, ref, toValue, watch } from 'vue'
import type { Component, MaybeRefOrGetter } from 'vue'
import { useQuery } from '@pinia/colada'
import { embedLoaders } from '#embed-manifest'
import type { Attachment, Message } from '../../shared/types/chat'
import type { components } from '../../shared/generated/api'
import type { EmbedContext } from '../types/embed'
import {
  attachmentMediaAsset,
  linkMediaExpirations,
  publicMediaURL,
} from '../../shared/utils/link-media'
import { linkMediaQuery } from '../queries/link-media'
import { embedContainerStyle, embedMatches, embedStyle, selectEmbed } from '../utils/embed'
import { offlineStatus } from '../utils/offline-archive'
import EmbedPlaceholder from '../components/EmbedPlaceholder.vue'

const renderers = new Map<string, Component>()
function rendererFor(component: string) {
  if (!renderers.has(component)) {
    const loaders = embedLoaders as Record<string, () => Promise<{ default: Component }>>
    renderers.set(
      component,
      defineAsyncComponent({
        loader: () => loaders[component]!().then((module) => module.default),
        loadingComponent: EmbedPlaceholder,
        errorComponent: EmbedPlaceholder,
        delay: 0,
        timeout: 15_000,
      }),
    )
  }
  return renderers.get(component)!
}
export interface UseEmbedInput {
  url: MaybeRefOrGetter<string>
  message: MaybeRefOrGetter<Pick<Message, 'id' | 'conversationId' | 'delivery'> | undefined>
  nearby: MaybeRefOrGetter<boolean>
  attachment?: MaybeRefOrGetter<Attachment | undefined>
  attachments?: MaybeRefOrGetter<Attachment[]>
  surface?: 'timeline' | 'catalog'
  onReady?: (url: string, ready: boolean) => void
}
/** Centralizes selection, queue/cache ownership, deadlines and offline states; providers never fetch. */
export function useEmbed(input: UseEmbedInput) {
  const auth = useAuthStore()
  const url = computed(() => toValue(input.url))
  const attachment = computed(
    () =>
      toValue(input.attachment) ??
      (toValue(input.attachments)?.length === 1 ? toValue(input.attachments)?.[0] : undefined),
  )
  const uploads = computed(
    () => toValue(input.attachments) ?? (attachment.value ? [attachment.value] : []),
  )
  const upload = computed(() =>
    attachment.value
      ? (attachmentMediaAsset(attachment.value, window.location.href) ?? undefined)
      : undefined,
  )
  const selection = computed(() =>
    selectEmbed(
      url.value,
      uploads.value.length > 1
        ? 'gallery'
        : attachment.value
          ? (upload.value?.kind ?? 'file')
          : undefined,
      upload.value?.contentType,
    ),
  )
  const surface = input.surface ?? 'timeline'
  const style = computed(() => embedStyle(selection.value, surface))
  const containerStyle = computed(() => embedContainerStyle(selection.value, surface))
  const renderer = computed(() => rendererFor(selection.value.provider.component))
  const viewerOpen = ref(false)
  const failed = ref(false)
  const isReady = ref(false)
  const nearby = computed(() => toValue(input.nearby) || viewerOpen.value)
  const identity = computed(() => ({
    account: auth.account?.id ?? '',
    conversation: toValue(input.message)?.conversationId ?? '',
    message: toValue(input.message)?.id ?? '',
    url: url.value,
    widget: false,
  }))
  const delivery = computed(() => toValue(input.message)?.delivery)
  // Keep the frame key independent of response identity/expiry to preserve a playing player.
  const widgetKey = computed(() =>
    JSON.stringify([
      identity.value.account,
      identity.value.conversation,
      identity.value.message,
      url.value,
    ]),
  )
  const enabled = computed(
    () =>
      selection.value.valid &&
      selection.value.provider.resolver === 'message' &&
      !uploads.value.length &&
      !!identity.value.message &&
      !!identity.value.conversation &&
      !delivery.value &&
      nearby.value &&
      !offlineStatus.value.offline,
  )
  const query = useQuery(() => linkMediaQuery({ ...identity.value, enabled: enabled.value }))
  const pending = computed(
    () => delivery.value === 'pending' || query.asyncStatus.value === 'loading',
  )
  const content = computed(() => {
    const value = query.data.value
    const provider = selection.value.provider
    // A late response cannot change the selected layout, adapter, or response family.
    if (
      delivery.value ||
      !value ||
      !provider.accepts.includes(value.kind) ||
      (provider.providers && !provider.providers.includes(value.provider))
    )
      return undefined
    if (value.kind === 'widget') {
      const source = publicMediaURL(value.widget?.url ?? '')
      if (
        !source ||
        !value.widget?.external ||
        !provider.frames?.some((rule) => embedMatches(rule, new URL(source)))
      )
        return undefined
    }
    return value
  })
  const { expired } = useLinkMediaExpiry({
    key: widgetKey,
    assets: () => linkMediaExpirations(query.data.value),
    enabled,
    pending,
    refresh: () => query.refetch(),
  })
  const saved = useOfflineMedia(url)
  const offlineAsset = computed<components['schemas']['LinkMediaAsset'] | undefined>(() => {
    const kind = saved.contentType.value.split('/')[0]
    return saved.saved.value && (kind === 'image' || kind === 'video' || kind === 'audio')
      ? { url: url.value, kind, contentType: saved.contentType.value }
      : undefined
  })
  const assets = computed(() => {
    if (failed.value) return []
    if (uploads.value.length)
      return uploads.value
        .map((attachment) => attachmentMediaAsset(attachment, window.location.href))
        .filter((asset) => asset !== null)
    if (offlineStatus.value.offline)
      return selection.value.provider.accepts.includes('media') && offlineAsset.value
        ? [offlineAsset.value]
        : []
    if (expired.value || content.value?.kind !== 'media') return []
    return content.value.assets
      .map((asset) => ({ ...asset, url: publicMediaURL(asset.url) }))
      .filter((asset) => asset.url)
  })
  const host = computed(() => {
    try {
      return new URL(url.value).hostname.replace(/^www\./, '')
    } catch {
      return ''
    }
  })
  const title = computed(
    () =>
      attachment.value?.alt ||
      attachment.value?.name ||
      query.data.value?.file?.name ||
      query.data.value?.title ||
      host.value,
  )
  const card = computed<components['schemas']['LinkMediaCard']>(() => ({
    site: host.value,
    title: title.value,
    description: query.data.value?.description || query.data.value?.file?.contentType,
    ...query.data.value?.card,
    reason:
      failed.value || query.error.value
        ? 'fetch_failed'
        : query.data.value?.card?.reason || query.data.value?.file?.reason,
  }))
  function ready(value: boolean) {
    isReady.value = value
  }
  function unavailable() {
    failed.value = true
    isReady.value = false
  }
  function viewing(value: boolean) {
    viewerOpen.value = value
  }
  watch(
    widgetKey,
    () => {
      failed.value = false
      isReady.value = false
      viewerOpen.value = false
    },
    { flush: 'sync' },
  )
  watch(
    () => content.value?.assets.map((asset) => asset.url).join('\n'),
    () => {
      failed.value = false
      isReady.value = false
    },
  )
  watch(isReady, (value) => input.onReady?.(url.value, value))
  onBeforeUnmount(() => input.onReady?.(url.value, false))
  const context = computed<EmbedContext>(() => ({
    url: url.value,
    host: host.value,
    selection: selection.value,
    surface,
    content: content.value,
    assets: assets.value,
    widget:
      !query.error.value && !offlineStatus.value.offline && content.value?.kind === 'widget'
        ? content.value.widget
        : undefined,
    file:
      attachment.value && !upload.value
        ? {
            name: attachment.value.name,
            url: publicMediaURL(attachment.value.url),
            contentType: attachment.value.contentType || 'application/octet-stream',
            size: attachment.value.size ?? 0,
          }
        : content.value?.kind === 'file' && !offlineStatus.value.offline
          ? content.value.file
          : undefined,
    card: card.value,
    title: title.value,
    attachment: attachment.value,
    downloadSource: saved.saved.value
      ? saved.source.value
      : attachment.value
        ? saved.source.value
        : assets.value[0]?.url,
    nearby: nearby.value,
    pending: pending.value,
    expired: expired.value,
    offline: offlineStatus.value.offline,
    failed: delivery.value === 'failed' || failed.value || !!query.error.value,
    widgetKey: widgetKey.value,
    ready,
    unavailable,
    viewing,
  }))
  return { selection, style, containerStyle, renderer, context, content, assets, expired, pending }
}
