<script setup lang="ts">
import type { components } from '../../shared/generated/api'
import { widgetSource, widgetHeight, providerWidgetHeight, widgetPlayerFailed } from '../../shared/utils/link-media'
const props = defineProps<{ widget: components['schemas']['LinkMediaWidget']; title: string }>()
const frame = ref<HTMLIFrameElement | null>(null)
const height = ref(Math.max(100, Math.min(800, props.widget.height)))
const failed = ref(false)
const activated = ref(false)
const posterFailed = ref(false)
const retrying = ref(false)
const retryVersion = ref(0)
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
  const value=widgetSource(props.widget, config.public.apiBase, window.location.href)
  if (!value) return value
  const url = new URL(value)
  if (!props.widget.external || url.hostname === 'platform.twitter.com' && url.pathname === '/embed/Tweet.html')
    url.searchParams.set('theme', appearance.mode)
  if (props.widget.external && url.hostname === 'embed.bsky.app' && url.pathname.startsWith('/embed/'))
    url.searchParams.set('colorMode', appearance.mode)
  return url.href
})
const dimensions = computed(() => props.widget.external && Number.isFinite(props.widget.aspectRatio) && (props.widget.aspectRatio ?? 0) > 0
  ? { aspectRatio: String(props.widget.aspectRatio), height: 'auto' }
  : undefined)
const sandbox = computed(() => props.widget.external
  ? 'allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox allow-presentation'
  : 'allow-scripts allow-popups allow-popups-to-escape-sandbox allow-presentation')
function resize(event: MessageEvent) {
  if(widgetPlayerFailed(event,frame.value?.contentWindow??null,source.value)) { failed.value=true;retrying.value=false;clearTimeout(retryTimer);return }
  const next = props.widget.external
    ? providerWidgetHeight(event, frame.value?.contentWindow ?? null, source.value)
    : widgetHeight(event, frame.value?.contentWindow ?? null, props.widget.token)
  if (next !== null) height.value = next
}
const failureText=computed(()=>{
  try{if(new URL(source.value).hostname.endsWith('tiktok.com')) return 'TikTok couldn’t serve this video.'}catch{}
  return 'This embed couldn’t be displayed.'
})
function retryPlayer() {
  if(retrying.value) return
  retrying.value=true;failed.value=false;activated.value=true;retryVersion.value++;retryStarted=Date.now()
  clearTimeout(retryTimer)
  retryTimer=setTimeout(()=>{retrying.value=false;failed.value=true},15000)
}
function loaded() {
  if(!retrying.value) return
  clearTimeout(retryTimer)
  retryTimer=setTimeout(()=>retrying.value=false,Math.max(0,300-(Date.now()-retryStarted)))
}
onMounted(() => window.addEventListener('message', resize))
onBeforeUnmount(() => {window.removeEventListener('message', resize);clearTimeout(retryTimer)})
</script>
<template>
  <div class="link-widget">
    <button v-if="source && widget.clickToLoad && !activated" type="button" class="widget-poster" :style="dimensions" aria-label="Load video player" @click="activated = true">
      <img v-if="widget.thumbnail && !posterFailed" :src="widget.thumbnail" alt="" referrerpolicy="no-referrer" @error="posterFailed = true" />
      <span class="widget-activate"><AppIcon name="play" :size="20" />Load video</span>
    </button>
    <iframe v-else-if="source && !failed" ref="frame" :key="`${source}:${retryVersion}`" :src="source" :title="title" :height="height" :style="dimensions" :sandbox="sandbox" allow="encrypted-media; fullscreen; picture-in-picture" loading="lazy" :referrerpolicy="widget.external ? 'strict-origin-when-cross-origin' : 'no-referrer'" @load="loaded" @error="failed = true; retrying = false" />
    <div v-else class="link-state link-failed"><span>{{ failureText }}</span><button type="button" class="link-retry" :disabled="retrying" @click="retryPlayer">Retry</button></div>
    <div v-if="retrying" class="widget-pending" role="status"><AppIcon name="refresh" :size="20" class="retry-spinner" />Retrying player…</div>
  </div>
</template>
