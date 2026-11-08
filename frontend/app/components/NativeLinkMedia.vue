<script setup lang="ts">
import type { components } from '../../shared/generated/api'
import { nativePostText, nativeStageRatio, publicMediaURL } from '../../shared/utils/link-media'
const props = defineProps<{ content: components['schemas']['LinkMediaResponse']; nearby: boolean }>()
const emit = defineEmits<{ unavailable: [] }>()
const index = ref(0)
const expanded = ref(false)
const failedAssets = reactive(new Set<string>())
const retryingAssets = reactive(new Set<string>())
const loadedAssets = reactive(new Set<string>())
/** Slides already shown keep their media, so paging back never reloads. */
const visited = reactive(new Set<number>([0]))
const captionExpanded = ref(false)
const caption = ref<HTMLElement | null>(null)
const track = ref<HTMLElement | null>(null)
const captionId = useId()
const captionTruncated = ref(false)
let captionObserver: ResizeObserver | undefined
function measureCaption() {
  if (caption.value && !captionExpanded.value)
    captionTruncated.value = caption.value.scrollHeight > caption.value.clientHeight + 1
}
onMounted(() => {
  captionObserver = new ResizeObserver(measureCaption)
  watch(caption, element => {
    captionObserver?.disconnect()
    if (element) captionObserver?.observe(element)
    measureCaption()
  }, { immediate: true, flush: 'post' })
})
watch(captionExpanded, () => nextTick(measureCaption))
const players = ref<HTMLMediaElement[]>([])
const assets = computed(() => props.content.assets.filter(asset => publicMediaURL(asset.url)))
const asset = computed(() => assets.value[index.value])
const source = computed(() => publicMediaURL(props.content.url))
const provider = computed(() => ({ reddit: 'Reddit', instagram: 'Instagram', tiktok: 'TikTok' }[props.content.provider] ?? props.content.provider))
const text = computed(() => nativePostText(props.content))
const label = computed(() => text.value.title || text.value.caption.split('\n')[0]?.slice(0, 120) || `${provider.value} post`)
/** Natural sizes learned from the first loaded item when the provider sent none. */
const learned = ref<{ width: number; height: number }>()
const ratio = computed(() => nativeStageRatio(assets.value.some(a => a.width && a.height) ? assets.value : learned.value ? [learned.value] : [], props.content.provider === 'reddit' ? 1 : 4 / 5))
const gallery = computed(() => assets.value.length > 1)
const dots = computed(() => assets.value.length <= 10)
watch(() => props.content, () => {
  index.value = 0; expanded.value = false; captionExpanded.value = false
  failedAssets.clear(); retryingAssets.clear(); loadedAssets.clear(); visited.clear(); visited.add(0)
  track.value?.scrollTo({ left: 0 })
})
watch(() => props.nearby, visible => { if (!visible) pause() })
onBeforeUnmount(() => { pause(); captionObserver?.disconnect(); clearTimeout(pagingTimer); cancelAnimationFrame(scrollFrame) })
function pause() { for (const player of players.value) player?.pause() }
const near = (i: number) => Math.abs(i - index.value) <= 1 || visited.has(i)
watch(index, i => { visited.add(i); pause() })

function go(to: number) {
  const next = Math.max(0, Math.min(assets.value.length - 1, to))
  const el = track.value
  if (!el) { index.value = next; return }
  const reduced = document.documentElement.dataset.motion === 'reduced' || matchMedia('(prefers-reduced-motion: reduce)').matches
  paging = next
  clearTimeout(pagingTimer)
  pagingTimer = setTimeout(() => (paging = undefined), 1000)
  el.scrollTo({ left: next * el.clientWidth, behavior: reduced ? 'auto' : 'smooth' })
  index.value = next
}
/** A slide the buttons or keys are scrolling to: the positions passed on the way aren't choices. */
let paging: number | undefined
let pagingTimer: ReturnType<typeof setTimeout> | undefined
function move(by: number) { go(index.value + by) }
// Paging inside the viewer leaves the card on the same item when it closes.
watch(index, i => {
  const el = track.value
  if (expanded.value && el?.clientWidth && Math.round(el.scrollLeft / el.clientWidth) !== i) el.scrollTo({ left: i * el.clientWidth })
})
let scrollFrame = 0
function onTrackScroll() {
  cancelAnimationFrame(scrollFrame)
  scrollFrame = requestAnimationFrame(() => {
    const el = track.value
    if (!el?.clientWidth) return
    if (paging !== undefined) {
      if (Math.abs(el.scrollLeft - paging * el.clientWidth) > 2) return
      paging = undefined
      clearTimeout(pagingTimer)
    }
    index.value = Math.max(0, Math.min(assets.value.length - 1, Math.round(el.scrollLeft / el.clientWidth)))
  })
}
async function failed(event: Event) {
  const url = (event.currentTarget as HTMLElement | null)?.getAttribute('src') ?? asset.value?.url
  if (url && retryingAssets.has(url)) await new Promise(resolve => setTimeout(resolve, 300))
  if (url) retryingAssets.delete(url)
  if (assets.value.length > 1 && url) failedAssets.add(url)
  else emit('unavailable')
}
function retryAsset() {
  if (!asset.value || retryingAssets.has(asset.value.url)) return
  retryingAssets.add(asset.value.url); failedAssets.delete(asset.value.url)
}
function loaded(event: Event) {
  const element = event.currentTarget as HTMLImageElement | HTMLVideoElement | null
  const url = element?.getAttribute('src') ?? asset.value?.url
  if (!url) return
  loadedAssets.add(url)
  setTimeout(() => retryingAssets.delete(url), 300)
  if (!learned.value && element) {
    const width = 'naturalWidth' in element ? element.naturalWidth : element.videoWidth
    const height = 'naturalHeight' in element ? element.naturalHeight : element.videoHeight
    if (width && height) learned.value = { width, height }
  }
}
function keys(event: KeyboardEvent) {
  if (event.key === 'ArrowLeft') { event.preventDefault(); move(-1) }
  if (event.key === 'ArrowRight') { event.preventDefault(); move(1) }
}
</script>
<template>
  <section class="native-link-media" :data-provider="content.provider" :aria-label="`${provider} post`" :style="{ '--media-ratio': ratio }">
    <header class="native-media-head">
      <span class="native-media-provider">{{ provider }}</span>
      <span v-if="text.name || text.handle" class="native-media-byline">
        <span v-if="text.name" class="native-media-name">{{ text.name }}</span>
        <span v-if="text.handle" class="native-media-handle">{{ text.handle }}</span>
      </span>
      <a class="native-media-open" :href="source" target="_blank" rel="noopener noreferrer" :aria-label="`Open on ${provider}`" :title="`Open on ${provider}`"><AppIcon name="external" :size="14" /></a>
    </header>
    <div v-if="text.title || text.caption" class="native-media-copy">
      <a v-if="text.title" class="native-media-title" :href="source" target="_blank" rel="noopener noreferrer">{{ text.title }}</a>
      <p v-if="text.caption" :id="captionId" ref="caption" class="native-media-caption" :class="{ 'is-expanded': captionExpanded }">{{ text.caption }}</p>
      <button v-if="captionTruncated || captionExpanded" type="button" class="native-media-more" :aria-controls="captionId" :aria-expanded="captionExpanded" @click="captionExpanded = !captionExpanded">{{ captionExpanded ? 'Show less' : 'Read more' }}</button>
    </div>
    <div v-if="asset" class="native-media-stage" :class="{ 'is-gallery': gallery }" :aria-roledescription="gallery ? 'carousel' : undefined" @keydown="keys">
      <div ref="track" class="native-media-track" @scroll.passive="onTrackScroll">
        <div v-for="(item, i) in assets" :key="item.url" class="native-media-slide" :class="{ 'is-current': i === index, 'is-loaded': loadedAssets.has(item.url) }" :aria-hidden="i === index ? undefined : 'true'" :inert="i !== index || undefined">
          <template v-if="near(i)">
            <div v-if="failedAssets.has(item.url)" class="native-media-unavailable" role="status">
              <AppIcon name="image" :size="28" />
              <span>This media couldn’t be loaded.</span>
              <button type="button" class="link-retry" @click="retryAsset">Retry</button>
            </div>
            <template v-else-if="item.kind === 'image'">
              <img class="native-media-backdrop" :src="item.url" alt="" aria-hidden="true" referrerpolicy="no-referrer" decoding="async" />
              <button type="button" class="native-media-image" :aria-label="`Open image ${i + 1} of ${assets.length}`" @click="index = i; expanded = true">
                <img class="native-media-photo" :src="item.url" :alt="label" :width="item.width || undefined" :height="item.height || undefined" :loading="i === index ? 'eager' : 'lazy'" decoding="async" referrerpolicy="no-referrer" @load="loaded" @error="failed" />
              </button>
            </template>
            <video v-else-if="item.kind === 'video'" ref="players" class="native-media-video" :src="item.url" controls playsinline preload="metadata" :aria-label="label" @loadedmetadata="loaded" @error="failed" />
            <audio v-else-if="item.kind === 'audio'" ref="players" :src="item.url" controls preload="metadata" :aria-label="label" @loadedmetadata="loaded" @error="failed" />
          </template>
        </div>
      </div>
      <div v-if="retryingAssets.has(asset.url)" class="native-media-wait" role="status"><AppIcon name="refresh" :size="20" class="retry-spinner" />Retrying media…</div>
      <template v-if="gallery">
        <button v-if="index > 0" type="button" class="native-media-nav is-previous" aria-label="Previous media" @click="move(-1)"><AppIcon name="left" :size="18" /></button>
        <button v-if="index < assets.length - 1" type="button" class="native-media-nav is-next" aria-label="Next media" @click="move(1)"><AppIcon name="right" :size="18" /></button>
        <span class="native-media-count" role="status">{{ index + 1 }} / {{ assets.length }}</span>
        <div v-if="dots" class="native-media-dots" aria-hidden="true">
          <span v-for="(item, i) in assets" :key="item.url" :class="{ 'is-current': i === index }" />
        </div>
      </template>
    </div>
    <MediaViewer v-if="expanded" v-model:index="index" :items="assets" :label="label" :heading="text.title || text.name || text.handle || `${provider} post`" :detail="[text.title ? text.name : '', text.handle, provider].filter(Boolean).join(' · ')" :source="source" :source-label="`View on ${provider}`" @close="expanded = false" />
  </section>
</template>
