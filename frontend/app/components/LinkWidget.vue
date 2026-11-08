<script setup lang="ts">
import type { components } from '../../shared/generated/api'
import {
  widgetSource,
  widgetHeight,
  providerWidgetHeight,
  widgetPlayerFailed,
} from '../../shared/utils/link-media'
const props = defineProps<{
  widget: components['schemas']['LinkMediaWidget']
  title: string
  href?: string
}>()
/**
 * Posts that size themselves after loading (X, Bluesky, script embeds in our own document) sit in a
 * fixed frame so the timeline never moves; the whole post opens in a dialog. Players with a declared
 * size or aspect ratio keep it.
 */
const framed = computed(() => {
  if (!props.widget.external) return true
  try {
    return ['platform.twitter.com', 'embed.bsky.app'].includes(new URL(props.widget.url).hostname)
  } catch {
    return false
  }
})
const full = ref(false)
const frame = ref<HTMLIFrameElement | null>(null)
const height = ref(Math.max(100, Math.min(800, props.widget.height)))
const failed = ref(false)
const activated = ref(false)
const posterFailed = ref(false)
const retrying = ref(false)
const retryVersion = ref(0)
function frameFailed() {
  failed.value = true
  retrying.value = false
}
let retryTimer: ReturnType<typeof setTimeout> | undefined
let retryStarted = 0
const config = useRuntimeConfig()
const appearance = useAppearanceStore()
watch([() => props.widget.url, () => props.widget.height], () => {
  clearTimeout(retryTimer)
  retrying.value = false
  height.value = Math.max(100, Math.min(800, props.widget.height))
  failed.value = false
  activated.value = false
  posterFailed.value = false
})
const source = computed(() => {
  const value = widgetSource(props.widget, config.public.apiBase, window.location.href)
  if (!value) return value
  const url = new URL(value)
  if (
    !props.widget.external ||
    (url.hostname === 'platform.twitter.com' && url.pathname === '/embed/Tweet.html')
  )
    url.searchParams.set('theme', appearance.mode)
  if (
    props.widget.external &&
    url.hostname === 'embed.bsky.app' &&
    url.pathname.startsWith('/embed/')
  )
    url.searchParams.set('colorMode', appearance.mode)
  return url.href
})
// A tall player (a YouTube Short) narrows rather than grow past the timeline's media height.
const dimensions = computed(() =>
  props.widget.external &&
  Number.isFinite(props.widget.aspectRatio) &&
  (props.widget.aspectRatio ?? 0) > 0
    ? {
        aspectRatio: String(props.widget.aspectRatio),
        height: 'auto',
        maxWidth: `${Math.round(440 * props.widget.aspectRatio!)}px`,
      }
    : undefined,
)
const sandbox = computed(() =>
  props.widget.external
    ? 'allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox allow-presentation'
    : 'allow-scripts allow-popups allow-popups-to-escape-sandbox allow-presentation',
)
function resize(event: MessageEvent) {
  if (widgetPlayerFailed(event, frame.value?.contentWindow ?? null, source.value)) {
    failed.value = true
    retrying.value = false
    clearTimeout(retryTimer)
    return
  }
  const next = props.widget.external
    ? providerWidgetHeight(event, frame.value?.contentWindow ?? null, source.value)
    : widgetHeight(event, frame.value?.contentWindow ?? null, props.widget.token)
  if (next !== null) height.value = next
}
const failureText = computed(() => {
  try {
    if (new URL(source.value).hostname.endsWith('tiktok.com'))
      return 'TikTok couldn’t serve this video.'
  } catch {}
  return 'This embed couldn’t be displayed.'
})
function retryPlayer() {
  if (retrying.value) return
  retrying.value = true
  failed.value = false
  activated.value = true
  retryVersion.value++
  retryStarted = Date.now()
  clearTimeout(retryTimer)
  retryTimer = setTimeout(() => {
    retrying.value = false
    failed.value = true
  }, 15000)
}
function loaded() {
  if (!retrying.value) return
  clearTimeout(retryTimer)
  retryTimer = setTimeout(
    () => (retrying.value = false),
    Math.max(0, 300 - (Date.now() - retryStarted)),
  )
}
onMounted(() => window.addEventListener('message', resize))
onBeforeUnmount(() => {
  window.removeEventListener('message', resize)
  clearTimeout(retryTimer)
})
</script>
<template>
  <div class="link-widget w-full min-w-0 relative">
    <button
      v-if="source && widget.clickToLoad && !activated"
      type="button"
      class="widget-poster grid [place-items:center] relative w-full min-h-37.5 [aspect-ratio:16_/_9] overflow-hidden rounded-[12px] bg-card"
      :style="dimensions"
      aria-label="Load video player"
      @click="activated = true"
    >
      <img
        v-if="widget.thumbnail && !posterFailed"
        :src="widget.thumbnail"
        alt=""
        referrerpolicy="no-referrer"
        @error="posterFailed = true"
      />
      <span
        class="widget-activate relative inline-flex items-center gap-2 py-2.5 px-3.5 text-foreground bg-card border border-solid border-border rounded-md text-[13px]"
        ><AppIcon
          name="play"
          :size="20"
        />Load video</span
      >
    </button>
    <div
      v-else-if="source && !failed"
      :class="{ 'widget-frame': framed }"
    >
      <iframe
        ref="frame"
        :key="`${source}:${retryVersion}`"
        :src="source"
        :title="title"
        :height="height"
        :style="dimensions"
        :sandbox="sandbox"
        allow="encrypted-media; fullscreen; picture-in-picture"
        loading="lazy"
        :referrerpolicy="widget.external ? 'strict-origin-when-cross-origin' : 'no-referrer'"
        @load="loaded"
        @error="frameFailed"
      />
      <div
        v-if="framed"
        class="widget-frame-more absolute inset-[auto_0_0] flex justify-center gap-2 pt-8 pr-3 pb-3 pl-3 [background:linear-gradient(transparent,_var(--bg,_#0b0d12)_70%)]"
      >
        <button
          type="button"
          class="button"
          @click="full = true"
        >
          Show full post
        </button>
        <a
          v-if="href"
          :href="href"
          target="_blank"
          rel="noopener noreferrer"
          >Open<AppIcon
            name="external"
            :size="12"
        /></a>
      </div>
    </div>
    <div
      v-else
      class="link-state flex flex-wrap items-center gap-2 text-muted-foreground text-[12px] min-h-7 link-failed"
    >
      <span>{{ failureText }}</span
      ><button
        type="button"
        class="link-retry inline-flex items-center justify-center gap-1.5 min-h-9 py-1.5 px-3 bg-secondary border border-solid border-border rounded-sm"
        :disabled="retrying"
        @click="retryPlayer"
      >
        Retry
      </button>
    </div>
    <div
      v-if="retrying"
      class="widget-pending absolute inset-0 grid [place-content:center] [justify-items:center] gap-2 bg-card rounded-[12px] text-muted-foreground text-[13px]"
      role="status"
    >
      <AppIcon
        name="refresh"
        :size="20"
        class="retry-spinner [animation:link-retry-spin_1s_linear_infinite]"
      />Retrying player…
    </div>
    <AppDialog
      v-if="full"
      :title="title"
      wide
      @close="full = false"
    >
      <div class="widget-full grid gap-3 pt-0 pr-5 pb-5 pl-5">
        <iframe
          :src="source"
          :title="title"
          :height="height"
          :sandbox="sandbox"
          allow="encrypted-media; fullscreen; picture-in-picture"
          loading="lazy"
          :referrerpolicy="widget.external ? 'strict-origin-when-cross-origin' : 'no-referrer'"
        />
      </div>
    </AppDialog>
  </div>
</template>
