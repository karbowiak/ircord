<script setup lang="ts">
import type { CSSProperties } from 'vue'
import type { components } from '../../shared/generated/api'
import { publicMediaURL, widgetSource } from '../../shared/utils/link-media'

/**
 * A known provider's player in a box whose size comes only from the player adapter: an aspect
 * ratio (YouTube, Vimeo) or a fixed height (Spotify), or the parent's box when `fill` is set.
 * The consent poster, the player, its failure and retry all fill that same box, and the frame
 * never resizes itself, whatever the provider asks for.
 */
const props = defineProps<{
  widget: components['schemas']['LinkMediaWidget']
  title: string
  href?: string
  expired?: boolean
  /** Width over height from the adapter; takes precedence over the widget's own values. */
  ratio?: number
  /** Fixed height in pixels from the adapter, for players without a ratio. */
  height?: number
  /** Fill the parent's reserved box, such as the catalog's 16:9 preview. */
  fill?: boolean
}>()
const { t } = useI18n()
const config = useRuntimeConfig()
const preferences = usePreferencesStore()
const failed = ref(false)
const activated = ref(false)
const consented = ref(false)
const posterFailed = ref(false)
const version = ref(0)
const retrying = ref(false)
let retryTimer: ReturnType<typeof setTimeout> | undefined

/** The player endpoints the server accepts; any other frame shows the fallback card. */
function trustedPlayer(raw: string) {
  try {
    const url = new URL(raw)
    if (url.protocol !== 'https:' || (url.port && url.port !== '443')) return false
    const parts = url.pathname.replace(/^\//, '').split('/')
    const id = (value?: string) => !!value && /^[\w-]{1,200}$/.test(value)
    switch (url.hostname) {
      case 'www.youtube.com':
      case 'www.youtube-nocookie.com':
        return parts.length === 2 && parts[0] === 'embed' && id(parts[1])
      case 'player.vimeo.com':
        return parts.length === 2 && parts[0] === 'video' && /^\d{1,20}$/.test(parts[1] ?? '')
      case 'open.spotify.com':
        return (
          parts.length === 3 &&
          parts[0] === 'embed' &&
          ['track', 'album', 'playlist', 'artist', 'episode', 'show'].includes(parts[1] ?? '') &&
          id(parts[2])
        )
      default:
        return false
    }
  } catch {
    return false
  }
}
const source = computed(() => {
  if (!props.widget.external) return ''
  const value = widgetSource(props.widget, config.public.apiBase, window.location.href)
  return value && trustedPlayer(value) ? value : ''
})
const positive = (value?: number) =>
  typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : undefined
const box = computed<CSSProperties | undefined>(() => {
  if (props.fill) return undefined
  const ratio =
    positive(props.ratio) ?? (props.height ? undefined : positive(props.widget.aspectRatio))
  if (ratio) return { aspectRatio: String(ratio) }
  const height = positive(props.height) ?? positive(props.widget.height) ?? 270
  return { height: `${Math.max(100, Math.min(800, Math.round(height)))}px` }
})
function hasExpired() {
  return (
    props.expired || (!!props.widget.expiresAt && Date.parse(props.widget.expiresAt) <= Date.now())
  )
}
const metadataExpired = computed(hasExpired)
// A metadata deadline must not restart a player the user already opened.
const live = computed(() => activated.value && !failed.value)
const canLoad = computed(() => !!source.value && (!metadataExpired.value || live.value))
const requireClick = computed(() => !preferences.ready || preferences.preferences.askBeforeEmbeds)
const gated = computed(() => requireClick.value && !consented.value)
const poster = computed(
  () => !metadataExpired.value && publicMediaURL(props.widget.thumbnail || ''),
)
const sandbox =
  'allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox allow-presentation'
const original = computed(() => publicMediaURL(props.href || ''))
const fallback = computed<components['schemas']['LinkMediaCard']>(() => ({
  site: original.value ? new URL(original.value).hostname : t('workspaceUi.links.link'),
  title: props.title,
  reason: 'fetch_failed',
}))
watch(
  source,
  () => {
    clearTimeout(retryTimer)
    retrying.value = false
    failed.value = false
    activated.value = false
    consented.value = false
    posterFailed.value = false
  },
  { immediate: true },
)
watch(requireClick, (required, previous) => {
  if (required && !previous) {
    // Enabling the privacy choice also stops already mounted third-party frames.
    activated.value = false
    consented.value = false
    retrying.value = false
    clearTimeout(retryTimer)
  }
})
watch(poster, () => {
  posterFailed.value = false
})
function frameFailed() {
  clearTimeout(retryTimer)
  failed.value = true
  retrying.value = false
}
function loadEmbed() {
  if (!source.value || hasExpired()) return
  consented.value = true
  activated.value = true
}
function retryPlayer() {
  if (retrying.value || !source.value || hasExpired()) return
  retrying.value = true
  failed.value = false
  consented.value = true
  activated.value = true
  version.value++
  clearTimeout(retryTimer)
  retryTimer = setTimeout(frameFailed, 15_000)
}
function loaded() {
  clearTimeout(retryTimer)
  retrying.value = false
  activated.value = true
}
onBeforeUnmount(() => clearTimeout(retryTimer))
</script>
<template>
  <div
    class="link-widget group/media relative w-full min-w-0 overflow-hidden rounded-xl bg-card"
    :class="{ 'h-full': fill }"
    :style="box"
  >
    <button
      v-if="canLoad && !failed && gated"
      type="button"
      class="widget-poster absolute inset-0 grid place-items-center overflow-hidden"
      :aria-label="t('mediaUi.widget.loadEmbedNamed', { title })"
      @click="loadEmbed"
    >
      <NuxtImg
        v-if="poster && !posterFailed"
        :src="poster"
        alt=""
        referrerpolicy="no-referrer"
        @error="posterFailed = true"
      />
      <span
        class="relative inline-flex items-center gap-2 rounded-md bg-card px-3.5 py-2.5 text-[length:var(--ui-small)] text-foreground"
      >
        <AppIcon
          name="external"
          :size="20"
        />{{ t('mediaUi.widget.loadEmbed') }}
      </span>
    </button>
    <iframe
      v-else-if="canLoad && !failed"
      :key="`${source}:${version}`"
      class="absolute inset-0 h-full w-full"
      :src="source"
      :title="title"
      :sandbox="sandbox"
      allow="encrypted-media; fullscreen; picture-in-picture"
      loading="lazy"
      referrerpolicy="strict-origin-when-cross-origin"
      @load="loaded"
      @error="frameFailed"
    />
    <!-- The fallback card fills the player's box rather than taking its own height. -->
    <div
      v-else
      class="absolute inset-0 [--embed-height:100%]"
    >
      <LinkCard
        :card="fallback"
        :url="href || ''"
        :expired="metadataExpired"
      />
      <UiButton
        v-if="source && !metadataExpired"
        type="button"
        variant="secondary"
        size="sm"
        class="absolute end-3 bottom-3"
        :disabled="retrying"
        @click="retryPlayer"
      >
        {{ t('mediaUi.widget.retry') }}
      </UiButton>
    </div>
    <MediaLinkActions
      v-if="canLoad && !failed && href"
      :url="href"
      always-visible
      class="absolute end-2 top-2"
    />
    <span
      v-if="retrying"
      class="absolute bottom-3 start-3 rounded-md bg-card p-2 text-[length:var(--ui-caption)] text-muted-foreground"
      role="status"
      >{{ t('mediaUi.widget.retrying') }}</span
    >
  </div>
</template>
