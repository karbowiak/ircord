<script setup vapor lang="ts">
import type { CSSProperties } from 'vue'
import type { components } from '../../shared/generated/api'

type Asset = components['schemas']['LinkMediaAsset']

/**
 * Media from one link or post. A single item keeps the direct media presentation; several
 * photos and videos form a mosaic whose shape is known before anything loads, so loading and
 * failures never move the timeline. Each count has its own arrangement within a short, stable
 * box. Up to nine items are all shown; a larger set shows nine and a tenth slot that counts
 * every item from the tenth on. The viewer pages through every item.
 */
const props = defineProps<{
  assets: Asset[]
  nearby: boolean
  /** Describes the media for assistive technology, such as the post title. */
  label?: string
  /** The posted link, for copy and open actions. */
  url?: string
  heading?: string
  detail?: string
  sourceLabel?: string
  /** The catalog's fixed box: fill the parent and show at most four slots. */
  fill?: boolean
  /** No longer changes anything: every stage has a fixed height. Kept for existing callers. */
  compact?: boolean
  /** False when the surrounding embed shows its own copy and open actions. */
  actions?: boolean
  plain?: boolean
  /** Uploaded galleries open the selected file rather than a shared social-post URL. */
  individualSources?: boolean
}>()
const emit = defineEmits<{ unavailable: []; ready: [ready: boolean]; viewing: [open: boolean] }>()
const { t, n } = useI18n()
const name = computed(
  () => props.label || t('mediaUi.gallery.label', { count: n(props.assets.length) }),
)

const visual = computed(() => props.assets.filter((asset) => asset.kind !== 'audio'))
const audio = computed(() => props.assets.filter((asset) => asset.kind === 'audio'))
const single = computed(() => props.assets.length === 1)
/**
 * Every item is shown up to `all`; past that, `all` items and one more slot whose count includes
 * its own item, so nothing is skipped. A fixed box (the Links page) shows three and a fourth slot.
 */
const all = computed(() => (props.fill ? 4 : 9))
const shown = computed(() =>
  visual.value.length <= all.value
    ? visual.value
    : visual.value.slice(0, props.fill ? all.value : all.value + 1),
)
const more = computed(() =>
  visual.value.length > all.value ? visual.value.length - shown.value.length + 1 : 0,
)

type Layout = { areas: string[]; rows?: string }
/**
 * One arrangement per count, each filling the same fixed stage (about 450 by 280 pixels in a post
 * card), so the count never changes the height. Odd counts lead with a tall feature, even counts
 * with a pair or a grid, and no tile becomes a thin strip on a phone.
 */
const layouts: Record<number, Layout> = {
  1: { areas: ['a'] },
  2: { areas: ['a b'] },
  3: { areas: ['a a b', 'a a c'] },
  4: { areas: ['a b', 'c d'] },
  5: { areas: ['a a b c', 'a a d e'] },
  6: { areas: ['a a b', 'a a c', 'd e f'] },
  7: { areas: ['a a b c', 'a a d e', 'a a f g'] },
  8: { areas: ['a a a b b b', 'c c d d e e', 'f f g g h h'], rows: '3fr 2fr 2fr' },
  9: { areas: ['a a b c', 'a a d e', 'f g h i'] },
  10: { areas: ['a a b b', 'c d e f', 'g h i j'], rows: '3fr 2fr 2fr' },
}
/** The catalog's 16:9 box shows at most four slots, so the four-item layouts serve it too. */
const layout = computed<Layout>(() => layouts[shown.value.length] ?? layouts[1]!)
const mosaic = computed<CSSProperties>(() => {
  const { areas, rows } = layout.value
  const columns = areas[0]!.split(' ').length
  return {
    gridTemplateAreas: areas.map((row) => `"${row}"`).join(' '),
    gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
    gridTemplateRows: rows
      ? rows.replace(/(\d+fr)/g, 'minmax(0, $1)')
      : `repeat(${areas.length}, minmax(0, 1fr))`,
  }
})
/** The parent's reserved `--embed-height`, or the same fixed stage as single media. */
const stage = computed<CSSProperties | undefined>(() =>
  props.fill ? undefined : { height: 'var(--embed-height, clamp(220px, 75vw, 400px))' },
)
const cell = (position: number): CSSProperties => ({
  gridArea: String.fromCharCode(97 + position),
})

const index = ref(0)
const viewing = ref(false)
/** Items below the mosaic that have their own viewer open. */
const playersViewing = reactive(new Set<string>())
function playerViewing(url: string, open: boolean) {
  if (open) playersViewing.add(url)
  else playersViewing.delete(url)
}
watch(
  () => viewing.value || playersViewing.size > 0,
  (open) => emit('viewing', open),
)
function open(position: number) {
  index.value = position
  viewing.value = true
}

const loaded = reactive(new Set<string>())
const failed = reactive(new Set<string>())
const ready = computed(() => !single.value && loaded.size > 0)
watch(ready, (value) => emit('ready', value))
watch(
  () => props.assets.map((asset) => asset.url).join('\n'),
  () => {
    loaded.clear()
    failed.clear()
  },
)
function load(url: string) {
  failed.delete(url)
  loaded.add(url)
}
function fail(url: string) {
  loaded.delete(url)
  failed.add(url)
  if (props.assets.every((asset) => failed.has(asset.url))) emit('unavailable')
}
onBeforeUnmount(() => {
  if (!single.value) {
    emit('ready', false)
    emit('viewing', false)
  }
})
</script>
<template>
  <DirectLinkMedia
    v-if="single"
    :asset="assets[0]!"
    :nearby="nearby"
    :url="url"
    :description="label"
    :actions="actions"
    :plain="plain"
    @ready="emit('ready', $event)"
    @viewing="emit('viewing', $event)"
    @unavailable="emit('unavailable')"
  />
  <div
    v-else
    class="media-gallery group/media relative flex min-w-0 w-full flex-col gap-1.5 overflow-hidden"
    :class="{ 'h-full': fill }"
    :style="stage"
    role="group"
    :aria-label="name"
  >
    <div
      v-if="shown.length"
      class="media-gallery-mosaic grid min-h-0 min-w-0 w-full flex-1 gap-1 overflow-hidden rounded-[12px]"
      :style="mosaic"
    >
      <MediaGalleryTile
        v-for="(asset, i) in shown"
        :key="asset.url"
        :style="cell(i)"
        :asset="asset"
        :position="i + 1"
        :total="visual.length"
        :nearby="nearby || viewing"
        :label="name"
        :more="i === shown.length - 1 && more > 0 ? more : undefined"
        @open="open(i)"
        @load="load(asset.url)"
        @error="fail(asset.url)"
      />
    </div>
    <!-- Audio players keep their own fixed height, not the stage's reserved one. -->
    <div
      v-if="audio.length"
      class="grid shrink-0 gap-1.5 overflow-y-auto [--embed-height:initial]"
      :class="shown.length ? 'max-h-[50%]' : 'min-h-0 flex-1'"
    >
      <DirectLinkMedia
        v-for="asset in audio"
        :key="asset.url"
        :asset="asset"
        :nearby="nearby"
        :url="url"
        :description="label"
        :actions="false"
        @ready="$event ? load(asset.url) : loaded.delete(asset.url)"
        @viewing="playerViewing(asset.url, $event)"
        @unavailable="fail(asset.url)"
      />
    </div>
    <MediaLinkActions
      v-if="url && actions !== false"
      :url="url"
      :open-label="sourceLabel"
      always-visible
      class="absolute top-2 right-2 z-2"
    />
    <MediaViewer
      v-if="viewing"
      v-model:index="index"
      :items="visual"
      :label="name"
      :heading="heading"
      :detail="detail"
      :source="individualSources ? visual[index]?.url : url"
      :source-label="sourceLabel"
      @close="viewing = false"
    />
  </div>
</template>
