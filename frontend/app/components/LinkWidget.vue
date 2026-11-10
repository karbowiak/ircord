<script setup vapor lang="ts">
import type { CSSProperties } from 'vue'
import { useResizeObserver } from '@vueuse/core'
import type { components } from '../../shared/generated/api'
import type { EmbedMatch } from '../../shared/types/embed'
import { publicMediaURL, widgetSource } from '../../shared/utils/link-media'
import { embedMatches } from '../utils/embed'

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
  /** The selected provider's static frame policy; the Go resolver approves sources separately. */
  frames: EmbedMatch[]
  /** Width over height from the adapter; takes precedence over the widget's own values. */
  ratio?: number
  /** Fixed height in pixels from the adapter, for players without a ratio. */
  height?: number
  /** Fill the parent's reserved box, such as the catalog's 16:9 preview. */
  fill?: boolean
  /** Preserve a provider's readable player viewport while fitting its reserved outer box. */
  minFrameWidth?: number
}>()
const { t } = useI18n()
const config = useRuntimeConfig()
const preferences = usePreferencesStore()
const failed = ref(false)
const activated = ref(false)
const frameLoaded = ref(false)
const consented = ref(false)
const posterFailed = ref(false)
const version = ref(0)
const retrying = ref(false)
const frameHost = ref<HTMLElement>()
const frameSize = ref({ width: 0, height: 0 })
useResizeObserver(frameHost, ([entry]) => {
  if (entry) frameSize.value = entry.contentRect
})
const frameStyle = computed<CSSProperties | undefined>(() => {
  const { width, height } = frameSize.value
  const minimum = props.minFrameWidth
  if (!minimum || width <= 0 || height <= 0 || width >= minimum) return undefined
  const scale = width / minimum
  return {
    width: `${minimum}px`,
    height: `${height / scale}px`,
    maxWidth: 'none',
    maxHeight: 'none',
    transform: `scale(${scale})`,
    transformOrigin: 'top left',
  }
})
let retryTimer: ReturnType<typeof setTimeout> | undefined

/** Keep the frame inside its selected adapter; response data cannot grant frame capability. */
function trustedPlayer(raw: string) {
  try {
    const url = new URL(raw)
    return (
      url.protocol === 'https:' &&
      (!url.port || url.port === '443') &&
      !url.username &&
      !url.password &&
      props.frames.some((rule) => embedMatches(rule, url))
    )
  } catch {
    return false
  }
}
const source = computed(() => {
  if (!props.widget.external) return ''
  const value = widgetSource(props.widget, config.public.apiBase, window.location.href)
  if (!value || !trustedPlayer(value)) return ''
  const url = new URL(value)
  // Twitch checks the page hosting its player. The API can be reached through a different
  // local alias, so bind only this approved player's parent to the actual browser hostname.
  if (
    (url.hostname === 'clips.twitch.tv' && url.pathname === '/embed') ||
    (url.hostname === 'player.twitch.tv' && url.pathname === '/')
  )
    url.searchParams.set('parent', window.location.hostname)
  return url.href
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
const requireClick = computed(() => preferences.preferences.askBeforeEmbeds)
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
    frameLoaded.value = false
    consented.value = false
    posterFailed.value = false
  },
  { immediate: true },
)
watch(requireClick, (required, previous) => {
  if (required && !previous) {
    // Enabling the privacy choice also stops already mounted third-party frames.
    activated.value = false
    frameLoaded.value = false
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
  frameLoaded.value = false
  version.value++
  clearTimeout(retryTimer)
  retryTimer = setTimeout(frameFailed, 15_000)
}
function loaded() {
  clearTimeout(retryTimer)
  retrying.value = false
  activated.value = true
  frameLoaded.value = true
}
onBeforeUnmount(() => clearTimeout(retryTimer))
</script>
<template>
  <div
    ref="frameHost"
    class="link-widget group/media relative w-full min-w-0 overflow-hidden rounded-xl bg-card"
    :class="{ 'h-full': fill }"
    :style="box"
  >
    <div
      v-if="canLoad && !failed && !preferences.ready"
      class="absolute inset-0 grid place-items-center text-[length:var(--ui-small)] text-muted-foreground"
      role="status"
      aria-busy="true"
    >
      {{ t('mediaUi.linkPreview.loading') }}
    </div>
    <button
      v-else-if="canLoad && !failed && gated"
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
    <div
      v-else-if="canLoad && !failed"
      class="absolute inset-0 h-full w-full"
    >
      <div
        v-if="!frameLoaded"
        class="absolute inset-0 grid place-items-center overflow-hidden bg-card p-4"
        role="status"
        aria-busy="true"
      >
        <NuxtImg
          v-if="poster && !posterFailed"
          :src="poster"
          alt=""
          class="absolute inset-0 h-full w-full object-cover"
          decoding="async"
          referrerpolicy="no-referrer"
          @error="posterFailed = true"
        />
        <span
          class="relative grid max-w-full gap-1 rounded-md bg-card/95 px-3.5 py-2.5 text-center"
        >
          <span class="line-clamp-2 text-[length:var(--ui-small)] text-foreground">{{
            title
          }}</span>
          <span class="text-[length:var(--ui-caption)] text-muted-foreground">{{
            t('mediaUi.linkPreview.loading')
          }}</span>
        </span>
      </div>
      <iframe
        :key="`${source}:${version}`"
        class="absolute inset-0 h-full w-full"
        :class="{ 'opacity-0 pointer-events-none': !frameLoaded }"
        :style="frameStyle"
        :src="source"
        :title="title"
        :sandbox="sandbox"
        :tabindex="frameLoaded ? undefined : -1"
        :aria-hidden="!frameLoaded || undefined"
        allow="encrypted-media; fullscreen; picture-in-picture"
        loading="lazy"
        referrerpolicy="strict-origin-when-cross-origin"
        @load="loaded"
        @error="frameFailed"
      />
    </div>
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
