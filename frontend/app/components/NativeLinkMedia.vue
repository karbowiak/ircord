<script setup lang="ts">
import type { components } from '../../shared/generated/api'
import { nativePostText, nativeStageRatio, publicMediaURL } from '../../shared/utils/link-media'
const props = defineProps<{
  content: components['schemas']['LinkMediaResponse']
  nearby: boolean
}>()
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
const { t, n } = useI18n()
function measureCaption() {
  if (caption.value && !captionExpanded.value)
    captionTruncated.value = caption.value.scrollHeight > caption.value.clientHeight + 1
}
useResizeObserver(caption, measureCaption)
watch(caption, measureCaption, { flush: 'post' })
watch(captionExpanded, () => nextTick(measureCaption))
const players = ref<HTMLMediaElement[]>([])
const assets = computed(() => props.content.assets.filter((asset) => publicMediaURL(asset.url)))
const asset = computed(() => assets.value[index.value])
const source = computed(() => publicMediaURL(props.content.url))
const provider = computed(
  () =>
    ({ reddit: 'Reddit', instagram: 'Instagram', tiktok: 'TikTok' })[props.content.provider] ??
    props.content.provider,
)
const text = computed(() => nativePostText(props.content))
const label = computed(
  () =>
    text.value.title ||
    text.value.caption.split('\n')[0]?.slice(0, 120) ||
    t('mediaUi.nativeMedia.post', { provider: provider.value }),
)
/** Natural sizes learned from the first loaded item when the provider sent none. */
const learned = ref<{ width: number; height: number }>()
const ratio = computed(() =>
  nativeStageRatio(
    assets.value.some((a) => a.width && a.height)
      ? assets.value
      : learned.value
        ? [learned.value]
        : [],
    props.content.provider === 'reddit' ? 1 : 4 / 5,
  ),
)
const gallery = computed(() => assets.value.length > 1)
const dots = computed(() => assets.value.length <= 10)
watch(
  () => props.content,
  () => {
    index.value = 0
    expanded.value = false
    captionExpanded.value = false
    failedAssets.clear()
    retryingAssets.clear()
    loadedAssets.clear()
    visited.clear()
    visited.add(0)
    track.value?.scrollTo({ left: 0 })
  },
)
watch(
  () => props.nearby,
  (visible) => {
    if (!visible) pause()
  },
)
onBeforeUnmount(() => {
  pause()
  clearTimeout(pagingTimer)
  cancelAnimationFrame(scrollFrame)
})
function pause() {
  for (const player of players.value) player?.pause()
}
const near = (i: number) => Math.abs(i - index.value) <= 1 || visited.has(i)
watch(index, (i) => {
  visited.add(i)
  pause()
})

function expandImage(position: number) {
  index.value = position
  expanded.value = true
}
function go(to: number) {
  const next = Math.max(0, Math.min(assets.value.length - 1, to))
  const el = track.value
  if (!el) {
    index.value = next
    return
  }
  const reduced =
    document.documentElement.dataset.motion === 'reduced' ||
    matchMedia('(prefers-reduced-motion: reduce)').matches
  paging = next
  clearTimeout(pagingTimer)
  pagingTimer = setTimeout(() => (paging = undefined), 1000)
  el.scrollTo({ left: next * el.clientWidth, behavior: reduced ? 'auto' : 'smooth' })
  index.value = next
}
/** A slide the buttons or keys are scrolling to: the positions passed on the way aren't choices. */
let paging: number | undefined
let pagingTimer: ReturnType<typeof setTimeout> | undefined
function move(by: number) {
  go(index.value + by)
}
// Paging inside the viewer leaves the card on the same item when it closes.
watch(index, (i) => {
  const el = track.value
  if (expanded.value && el?.clientWidth && Math.round(el.scrollLeft / el.clientWidth) !== i)
    el.scrollTo({ left: i * el.clientWidth })
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
    index.value = Math.max(
      0,
      Math.min(assets.value.length - 1, Math.round(el.scrollLeft / el.clientWidth)),
    )
  })
}
async function failed(event: Event) {
  const url = (event.currentTarget as HTMLElement | null)?.getAttribute('src') ?? asset.value?.url
  if (url && retryingAssets.has(url)) await new Promise((resolve) => setTimeout(resolve, 300))
  if (url) retryingAssets.delete(url)
  if (assets.value.length > 1 && url) failedAssets.add(url)
  else emit('unavailable')
}
function retryAsset() {
  if (!asset.value || retryingAssets.has(asset.value.url)) return
  retryingAssets.add(asset.value.url)
  failedAssets.delete(asset.value.url)
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
  if (event.key === 'ArrowLeft') {
    event.preventDefault()
    move(-1)
  }
  if (event.key === 'ArrowRight') {
    event.preventDefault()
    move(1)
  }
}
</script>
<template>
  <section
    class="native-link-media relative grid overflow-hidden min-w-0 w-[min(100%,_420px)] border border-solid border-border rounded-[14px] bg-secondary [container-type:inline-size]"
    :data-provider="content.provider"
    :aria-label="t('mediaUi.nativeMedia.post', { provider })"
    :style="{ '--media-ratio': ratio }"
  >
    <header
      class="native-media-head flex items-center gap-2 min-w-0 min-h-10 pt-2 pr-2 pb-0 pl-3 text-[12px]"
    >
      <span
        class="native-media-provider flex-none inline-flex items-center gap-1.5 text-foreground font-[600]"
        >{{ provider }}</span
      >
      <span
        v-if="text.name || text.handle"
        class="native-media-byline flex items-baseline gap-1.5 min-w-0 overflow-hidden text-muted-foreground whitespace-nowrap"
      >
        <span
          v-if="text.name"
          class="native-media-name overflow-hidden text-ellipsis text-body font-[500]"
          >{{ text.name }}</span
        >
        <span
          v-if="text.handle"
          class="native-media-handle overflow-hidden text-ellipsis"
          >{{ text.handle }}</span
        >
      </span>
      <a
        class="native-media-open flex-none grid [place-items:center] w-7 h-7 ml-auto rounded-sm text-muted-foreground"
        :href="source"
        target="_blank"
        rel="noopener noreferrer"
        :aria-label="t('mediaUi.nativeMedia.openOn', { provider })"
        :title="t('mediaUi.nativeMedia.openOn', { provider })"
        ><AppIcon
          name="external"
          :size="14"
      /></a>
    </header>
    <div
      v-if="text.title || text.caption"
      class="native-media-copy relative block h-19 pt-1.5 pr-3 pb-2.5 pl-3 overflow-hidden"
      :class="{ 'is-expanded': captionExpanded }"
    >
      <a
        v-if="text.title"
        class="native-media-title block overflow-hidden text-foreground text-[14px] font-[600] leading-[20px] whitespace-nowrap text-ellipsis"
        :href="source"
        target="_blank"
        rel="noopener noreferrer"
        >{{ text.title }}</a
      >
      <p
        v-if="text.caption"
        :id="captionId"
        ref="caption"
        class="native-media-caption m-0 text-body text-[13px] leading-[20px] whitespace-pre-wrap wrap-anywhere [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:3] overflow-hidden"
        :class="{ 'is-expanded': captionExpanded }"
      >
        {{ text.caption }}
      </p>
      <button
        v-if="captionTruncated || captionExpanded"
        type="button"
        class="native-media-more absolute right-3 bottom-2.5 pl-7 text-primary text-[12px] leading-[20px] [background:linear-gradient(90deg,_transparent,_var(--raised)_24px)]"
        :aria-controls="captionId"
        :aria-expanded="captionExpanded"
        @click="captionExpanded = !captionExpanded"
      >
        {{
          captionExpanded ? $t('mediaUi.nativeMedia.showLess') : $t('mediaUi.nativeMedia.readMore')
        }}
      </button>
    </div>
    <div
      v-if="asset"
      class="native-media-stage relative w-full min-w-0 h-[var(--stage-max)] bg-[#0b0d12] overflow-hidden"
      :class="{ 'is-gallery': gallery }"
      :aria-roledescription="gallery ? $t('mediaUi.nativeMedia.carousel') : undefined"
      @keydown="keys"
    >
      <div
        ref="track"
        class="native-media-track flex h-full overflow-x-auto overflow-y-hidden [scroll-snap-type:x_mandatory] [overscroll-behavior-x:contain] [scrollbar-width:none]"
        @scroll.passive="onTrackScroll"
      >
        <div
          v-for="(item, i) in assets"
          :key="item.url"
          class="native-media-slide relative [flex:0_0_100%] h-full [scroll-snap-align:center] [scroll-snap-stop:always] flex items-center justify-center overflow-hidden"
          :class="{ 'is-current': i === index, 'is-loaded': loadedAssets.has(item.url) }"
          :aria-hidden="i === index ? undefined : 'true'"
          :inert="i !== index || undefined"
        >
          <template v-if="near(i)">
            <div
              v-if="failedAssets.has(item.url)"
              class="native-media-unavailable relative grid [justify-items:center] gap-2.5 p-5 text-muted-foreground text-[13px] text-center"
              role="status"
            >
              <AppIcon
                name="image"
                :size="28"
              />
              <span>{{ $t('mediaUi.nativeMedia.loadFailed') }}</span>
              <button
                type="button"
                class="link-retry inline-flex items-center justify-center gap-1.5 min-h-9 py-1.5 px-3 bg-secondary border border-solid border-border rounded-sm"
                @click="retryAsset"
              >
                {{ $t('mediaUi.nativeMedia.retry') }}
              </button>
            </div>
            <template v-else-if="item.kind === 'image'">
              <NuxtImg
                class="native-media-backdrop absolute inset-[-10%] w-[120%] h-[120%] object-cover [filter:blur(28px)_saturate(1.3)_brightness(0.55)] [transform:translateZ(0)] pointer-events-none"
                :src="item.url"
                alt=""
                aria-hidden="true"
                referrerpolicy="no-referrer"
                decoding="async"
              />
              <button
                type="button"
                class="native-media-image relative flex items-center justify-center w-full h-full p-0 bg-transparent cursor-zoom-in"
                :aria-label="
                  $t('mediaUi.nativeMedia.openImage', {
                    current: n(i + 1),
                    total: n(assets.length),
                  })
                "
                @click="expandImage(i)"
              >
                <!-- Native on purpose: the load and error handlers read currentTarget's src and
                     natural size, and failed/loaded assets are keyed by that exact URL. -->
                <img
                  class="native-media-photo block max-w-full max-h-full w-auto h-auto object-contain [opacity:0] [transition:opacity_0.2s_ease]"
                  :src="item.url"
                  :alt="label"
                  :width="item.width || undefined"
                  :height="item.height || undefined"
                  :loading="i === index ? 'eager' : 'lazy'"
                  decoding="async"
                  referrerpolicy="no-referrer"
                  @load="loaded"
                  @error="failed"
                />
              </button>
            </template>
            <video
              v-else-if="item.kind === 'video'"
              ref="players"
              class="native-media-video relative block w-full h-full object-contain bg-[#000]"
              :src="item.url"
              controls
              playsinline
              preload="metadata"
              :aria-label="label"
              @loadedmetadata="loaded"
              @error="failed"
            />
            <audio
              v-else-if="item.kind === 'audio'"
              ref="players"
              :src="item.url"
              controls
              preload="metadata"
              :aria-label="label"
              @loadedmetadata="loaded"
              @error="failed"
            />
          </template>
        </div>
      </div>
      <div
        v-if="retryingAssets.has(asset.url)"
        class="native-media-wait absolute inset-0 grid [place-content:center] [justify-items:center] gap-2.5 bg-card text-muted-foreground text-[13px]"
        role="status"
      >
        <AppIcon
          name="refresh"
          :size="20"
          class="retry-spinner [animation:link-retry-spin_1s_linear_infinite]"
        />{{ $t('mediaUi.nativeMedia.retrying') }}
      </div>
      <template v-if="gallery">
        <button
          v-if="index > 0"
          type="button"
          class="native-media-nav absolute top-1/2 z-1 grid [place-items:center] w-8.5 h-8.5 mt-[-17px] rounded-full text-[#fff] [background:rgb(12_14_20_/_62%)] [backdrop-filter:blur(8px)] shadow-[0_1px_4px_rgb(0_0_0_/_30%)] [opacity:0] [transition:opacity_0.15s_ease,_background_0.15s_ease] is-previous"
          :aria-label="$t('mediaUi.nativeMedia.previous')"
          @click="move(-1)"
        >
          <AppIcon
            name="left"
            :size="18"
          />
        </button>
        <button
          v-if="index < assets.length - 1"
          type="button"
          class="native-media-nav absolute top-1/2 z-1 grid [place-items:center] w-8.5 h-8.5 mt-[-17px] rounded-full text-[#fff] [background:rgb(12_14_20_/_62%)] [backdrop-filter:blur(8px)] shadow-[0_1px_4px_rgb(0_0_0_/_30%)] [opacity:0] [transition:opacity_0.15s_ease,_background_0.15s_ease] is-next"
          :aria-label="$t('mediaUi.nativeMedia.next')"
          @click="move(1)"
        >
          <AppIcon
            name="right"
            :size="18"
          />
        </button>
        <span
          class="native-media-count absolute top-2.5 right-2.5 z-1 py-0.5 px-2 rounded-[999px] text-[#fff] [background:rgb(12_14_20_/_62%)] [backdrop-filter:blur(8px)] text-[11px] font-[600] tabular-nums pointer-events-none"
          role="status"
          >{{
            $t('mediaUi.nativeMedia.itemCount', {
              current: n(index + 1),
              total: n(assets.length),
            })
          }}</span
        >
        <div
          v-if="dots"
          class="native-media-dots absolute bottom-2.5 left-1/2 z-1 flex gap-[5px] py-1 px-1.5 rounded-[999px] [background:rgb(12_14_20_/_45%)] [transform:translateX(-50%)] pointer-events-none"
          aria-hidden="true"
        >
          <span
            v-for="(item, i) in assets"
            :key="item.url"
            :class="{ 'is-current': i === index }"
          />
        </div>
      </template>
    </div>
    <MediaViewer
      v-if="expanded"
      v-model:index="index"
      :items="assets"
      :label="label"
      :heading="
        text.title || text.name || text.handle || t('mediaUi.nativeMedia.post', { provider })
      "
      :detail="[text.title ? text.name : '', text.handle, provider].filter(Boolean).join(' · ')"
      :source="source"
      :source-label="t('mediaUi.nativeMedia.viewOn', { provider })"
      @close="expanded = false"
    />
  </section>
</template>
