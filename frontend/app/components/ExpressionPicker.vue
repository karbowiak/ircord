<script setup lang="ts">
import { useInfiniteQuery, useQuery } from '@pinia/colada'
import type { EmojiItem, GifCategory, GifItem } from '../../shared/types/media'
import { emojiText, providerName, rankEmoji, searchEmoji } from '../../shared/utils/media'
import { layoutLimits } from '../stores/layout'
const props = defineProps<{ anchor: HTMLElement; tab: 'gifs' | 'emoji'; emojiOnly?: boolean }>()
const emit = defineEmits<{
  close: [restoreFocus: boolean]
  'update:tab': [tab: 'gifs' | 'emoji']
  emoji: [item: EmojiItem]
  gif: [item: GifItem, query: string]
}>()
const media = useMediaStore()
const library = useLibraryStore()
const layout = useLayoutStore()
const appearance = useAppearanceStore()
media.needAll()

const panel = ref<HTMLElement | null>(null)
const search = ref<HTMLInputElement | null>(null)
const gifQuery = ref('')
const debouncedGifQuery = ref('')
const emojiQuery = ref('')
const query = computed({
  get: () => (props.tab === 'gifs' ? gifQuery.value : emojiQuery.value),
  set: (value) => {
    if (props.tab === 'gifs') gifQuery.value = value
    else emojiQuery.value = value
  },
})
const gifView = ref<'home' | 'results' | 'favorites'>('home')
const categoryTitle = ref('Trending')
const emojiGroup = ref('frequent')
const preview = shallowRef<EmojiItem>()
const tonesOpen = ref(false)
const titleId = useId()
const gifsId = useId()
const emojiId = useId()
const bodyId = useId()
const tones = ['👋', '👋🏻', '👋🏼', '👋🏽', '👋🏾', '👋🏿']
const toneNames = ['default', 'light', 'medium-light', 'medium', 'medium-dark', 'dark']

// ---- GIF data: cached per search, so reopening or going back is instant -----------------------

let debounce: ReturnType<typeof setTimeout> | undefined
watch(gifQuery, (q) => {
  clearTimeout(debounce)
  debounce = setTimeout(() => {
    debouncedGifQuery.value = q.trim()
    // Switch views only once the search runs, so typing never flashes trending results.
    if (debouncedGifQuery.value && gifView.value === 'home') gifView.value = 'results'
  }, 300)
})
const categories = useQuery({
  key: ['klipy', 'categories'],
  query: ({ signal }) => klipyCategories(media.appKey, signal),
  enabled: () => media.gifsReady && props.tab === 'gifs',
  staleTime: 60 * 60_000,
  gcTime: false,
})
const gifResults = useInfiniteQuery({
  key: () => ['klipy', 'gifs', debouncedGifQuery.value],
  query: ({ pageParam, signal }) =>
    klipyGifs(media.appKey, debouncedGifQuery.value, pageParam, signal),
  initialPageParam: 1,
  getNextPageParam: (last) => (last.items.length ? last.nextPage : null),
  enabled: () => media.gifsReady && props.tab === 'gifs' && gifView.value === 'results',
  staleTime: 10 * 60_000,
  gcTime: 30 * 60_000,
})
const savedGifs = computed(() => {
  const q = gifQuery.value.trim().toLowerCase()
  return library.library.gifs.filter((gif) => !q || gif.title.toLowerCase().includes(q))
})
const gifItems = computed<GifItem[]>(() => {
  if (gifView.value === 'favorites') return savedGifs.value
  const seen = new Set<string>()
  return (gifResults.data.value?.pages ?? [])
    .flatMap((page) => page.items)
    .filter((item) => !seen.has(item.id) && !!seen.add(item.id))
    .slice(0, 240)
})
const gifLoading = computed(() =>
  gifView.value === 'home'
    ? categories.isLoading.value && !categories.data.value
    : gifView.value === 'results' && gifResults.isLoading.value && !gifItems.value.length,
)
const gifError = computed(() =>
  gifView.value === 'home'
    ? categories.error.value
    : gifView.value === 'results'
      ? gifResults.error.value
      : null,
)
// Two balanced columns, like a masonry grid, using each GIF's real aspect ratio.
const gifColumns = computed(() => {
  const columns: { items: GifItem[]; height: number }[] = [
    { items: [], height: 0 },
    { items: [], height: 0 },
  ]
  for (const item of gifItems.value) {
    const shortest = columns[0]!.height <= columns[1]!.height ? columns[0]! : columns[1]!
    shortest.items.push(item)
    shortest.height += item.height / item.width
  }
  return columns.map((c) => c.items)
})
function gifScroll(event: Event) {
  const el = event.target as HTMLElement
  if (
    gifView.value === 'results' &&
    el.scrollHeight - el.scrollTop - el.clientHeight < 400 &&
    gifResults.hasNextPage.value &&
    !gifResults.isLoading.value &&
    gifItems.value.length < 240
  )
    void gifResults.loadNextPage()
}

function openFavorites() {
  gifQuery.value = ''
  gifView.value = 'favorites'
}
function trending() {
  gifQuery.value = ''
  debouncedGifQuery.value = ''
  categoryTitle.value = 'Trending'
  gifView.value = 'results'
}
function category(item: GifCategory) {
  categoryTitle.value = item.name
  gifQuery.value = item.query
  debouncedGifQuery.value = item.query
  gifView.value = 'results'
}
function back() {
  gifQuery.value = ''
  gifView.value = 'home'
}

// ---- Emoji ---------------------------------------------------------------------------------------

const groups = [
  { id: 'frequent', label: 'Frequently used', icon: 'clock' },
  { id: 'workspace', label: 'Server emoji', icon: 'brand' },
  { id: '7tv', label: '7TV global emotes', icon: 'globe' },
  { id: 'smileys-emotion', label: 'Smileys & emotion', icon: 'smile' },
  { id: 'people-body', label: 'People & body', icon: 'hand' },
  { id: 'animals-nature', label: 'Animals & nature', icon: 'leaf' },
  { id: 'food-drink', label: 'Food & drink', icon: 'food' },
  { id: 'travel-places', label: 'Travel & places', icon: 'car' },
  { id: 'activities', label: 'Activities', icon: 'trophy' },
  { id: 'objects', label: 'Objects', icon: 'lightbulb' },
  { id: 'symbols', label: 'Symbols', icon: 'heart' },
  { id: 'flags', label: 'Flags', icon: 'flag' },
]
const resolve = (ids: string[]) =>
  ids.map((id) => media.byId.get(id)).filter((item): item is EmojiItem => !!item)
const frequent = computed(() => resolve(library.frequentEmoji).slice(0, 36))
const displayedEmoji = computed(() => {
  if (emojiQuery.value.trim())
    return rankEmoji(
      searchEmoji(media.emojis, emojiQuery.value),
      library.usageRank,
      emojiQuery.value,
    )
  if (emojiGroup.value === 'frequent')
    return frequent.value.length
      ? frequent.value
      : media.standard.filter((e) => e.group === 'smileys-emotion').slice(0, 45)
  return searchEmoji(media.emojis, '', emojiGroup.value)
})
const emojiTitle = computed(() =>
  emojiQuery.value.trim()
    ? `${displayedEmoji.value.length} results`
    : emojiGroup.value === 'frequent' && !frequent.value.length
      ? 'Popular'
      : groups.find((g) => g.id === emojiGroup.value)?.label,
)
const emojiLoading = computed(
  () =>
    (media.standardQuery.isLoading && !media.standard.length) ||
    (media.catalog.isLoading && !media.custom.length),
)
function chooseEmoji(item: EmojiItem) {
  library.useEmoji(item)
  emit('emoji', item)
}
function selectGroup(id: string) {
  emojiGroup.value = id
  emojiQuery.value = ''
}
watch(emojiQuery, () => (preview.value = undefined))

// ---- Placement and resizing ------------------------------------------------------------------

const position = ref<Record<string, string>>({ visibility: 'hidden' })
const placedBelow = ref(false)
/** Phones get a bottom sheet instead of a popover. */
const sheet = ref(false)
function place() {
  const rect = props.anchor.getBoundingClientRect()
  const viewportHeight = window.visualViewport?.height ?? window.innerHeight
  const style = getComputedStyle(document.documentElement)
  let left = 0
  let right = window.innerWidth
  let top = 0
  let bottom = viewportHeight
  // A picker stays on the same physical display as the control that opened it.
  if (window.matchMedia('(horizontal-viewport-segments: 2)').matches) {
    const start = parseFloat(style.getPropertyValue('--segment-right-start'))
    const end = parseFloat(style.getPropertyValue('--segment-left-end'))
    if (rect.left >= start) left = start
    else right = end
  }
  if (window.matchMedia('(vertical-viewport-segments: 2)').matches) {
    const start = parseFloat(style.getPropertyValue('--segment-lower-start'))
    const end = parseFloat(style.getPropertyValue('--segment-upper-end'))
    if (rect.top >= start) top = Math.min(start, bottom)
    else bottom = Math.min(end, bottom)
  }
  const availableWidth = right - left
  const availableHeight = bottom - top
  sheet.value = availableWidth <= 700
  if (sheet.value) {
    placedBelow.value = false
    const height = Math.round(Math.min(availableHeight * 0.62, 520))
    position.value = {
      left: `${left}px`,
      top: `${bottom - height}px`,
      width: `${availableWidth}px`,
      height: `${height}px`,
    }
    return
  }
  const width = Math.min(layout.state.pickerWidth, availableWidth - 16)
  const above = rect.top - top - 16
  const below = bottom - rect.bottom - 16
  placedBelow.value = below > above && above < 360
  const height = Math.min(
    availableHeight - 16,
    Math.max(160, Math.min(layout.state.pickerHeight, placedBelow.value ? below : above)),
  )
  position.value = {
    left: `${Math.max(left + 8, Math.min(rect.right - width, right - width - 8))}px`,
    top: `${Math.max(top + 8, Math.min(placedBelow.value ? rect.bottom + 8 : rect.top - height - 8, bottom - height - 8))}px`,
    width: `${width}px`,
    height: `${height}px`,
  }
}
watch(() => [layout.state.pickerWidth, layout.state.pickerHeight], place)

let drag: { x: number; y: number; width: number; height: number } | undefined
function resizeStart(event: PointerEvent) {
  if (event.button !== 0) return
  event.preventDefault()
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
  const box = panel.value!.getBoundingClientRect()
  drag = { x: event.clientX, y: event.clientY, width: box.width, height: box.height }
  document.documentElement.classList.add('is-resizing')
}
function resizeMove(event: PointerEvent) {
  if (!drag) return
  layout.set('pickerWidth', drag.width + (drag.x - event.clientX))
  // The picker grows upward when above its anchor and downward when below it.
  layout.set(
    'pickerHeight',
    drag.height + (placedBelow.value ? event.clientY - drag.y : drag.y - event.clientY),
  )
}
function resizeEnd() {
  drag = undefined
  document.documentElement.classList.remove('is-resizing')
}
function resizeKey(event: KeyboardEvent) {
  const d = { ArrowLeft: [16, 0], ArrowRight: [-16, 0], ArrowUp: [0, 16], ArrowDown: [0, -16] }[
    event.key
  ]
  if (!d) return
  event.preventDefault()
  layout.set('pickerWidth', layout.state.pickerWidth + d[0]!)
  layout.set('pickerHeight', layout.state.pickerHeight + d[1]! * (placedBelow.value ? -1 : 1))
}
function resetSize() {
  layout.set('pickerWidth', layoutLimits.pickerWidth.default)
  layout.set('pickerHeight', layoutLimits.pickerHeight.default)
}

// ---- Focus and dismissal ---------------------------------------------------------------------

function outside(event: PointerEvent) {
  const target = event.target as Node
  if (!panel.value?.contains(target) && !props.anchor.contains(target)) emit('close', false)
}
function keys(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    emit('close', true)
  }
}
function switchTab(tab: 'gifs' | 'emoji') {
  emit('update:tab', tab)
  tonesOpen.value = false
  nextTick(() => search.value?.focus())
}
function tabKeys(event: KeyboardEvent) {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
  event.preventDefault()
  const next = props.tab === 'gifs' ? 'emoji' : 'gifs'
  switchTab(next)
  nextTick(() => document.getElementById(next === 'gifs' ? gifsId : emojiId)?.focus())
}
function clearSearch() {
  query.value = ''
  search.value?.focus()
}

let observer: ResizeObserver | undefined
onMounted(() => {
  panel.value?.showPopover?.()
  place()
  // The panel stays hidden until its placement renders, and hidden fields can't take focus.
  nextTick(() => search.value?.focus())
  document.addEventListener('pointerdown', outside)
  window.addEventListener('resize', place)
  window.visualViewport?.addEventListener('resize', place)
  observer = new ResizeObserver(place)
  observer.observe(props.anchor)
})
onBeforeUnmount(() => {
  clearTimeout(debounce)
  observer?.disconnect()
  document.removeEventListener('pointerdown', outside)
  window.removeEventListener('resize', place)
  window.visualViewport?.removeEventListener('resize', place)
  document.documentElement.classList.remove('is-resizing')
})
</script>
<template>
  <Teleport to="body">
    <div
      ref="panel"
      popover="manual"
      class="expression-picker fixed inset-auto m-0 p-0 flex flex-col overflow-hidden bg-card text-foreground border-0 rounded-lg shadow-popover z-40"
      :class="{ 'is-below': placedBelow, 'is-sheet': sheet }"
      :style="position"
      role="dialog"
      :aria-labelledby="titleId"
      @keydown="keys"
    >
      <span
        :id="titleId"
        class="sr-only"
        >{{ emojiOnly ? 'Choose a reaction' : 'GIF and emoji picker' }}</span
      >
      <div
        class="picker-resize absolute top-0 left-0 w-4.5 h-4.5 z-3 cursor-nwse-resize [touch-action:none]"
        role="separator"
        tabindex="0"
        aria-label="Resize picker"
        :aria-valuenow="layout.state.pickerWidth"
        :aria-valuemin="layoutLimits.pickerWidth.min"
        :aria-valuemax="layoutLimits.pickerWidth.max"
        title="Drag to resize · double-click to reset"
        @pointerdown="resizeStart"
        @pointermove="resizeMove"
        @pointerup="resizeEnd"
        @pointercancel="resizeEnd"
        @dblclick="resetSize"
        @keydown="resizeKey"
      />
      <header
        class="expression-header flex items-center gap-1.5 pt-2.5 pr-2.5 pb-2 pl-3.5 shrink-0"
      >
        <div
          v-if="!emojiOnly"
          class="expression-tabs flex gap-0.5 mr-1"
          role="tablist"
          aria-label="Expression type"
          @keydown="tabKeys"
        >
          <button
            :id="gifsId"
            role="tab"
            :aria-selected="tab === 'gifs'"
            :aria-controls="bodyId"
            :tabindex="tab === 'gifs' ? 0 : -1"
            :class="{ active: tab === 'gifs' }"
            @click="switchTab('gifs')"
          >
            GIFs</button
          ><button
            :id="emojiId"
            role="tab"
            :aria-selected="tab === 'emoji'"
            :aria-controls="bodyId"
            :tabindex="tab === 'emoji' ? 0 : -1"
            :class="{ active: tab === 'emoji' }"
            @click="switchTab('emoji')"
          >
            Emoji
          </button>
        </div>
        <UiButton
          v-if="tab === 'gifs' && gifView !== 'home'"
          variant="ghost"
          size="icon-sm"
          aria-label="Back to GIF categories"
          @click="back"
        >
          <AppIcon
            name="back"
            :size="16" /></UiButton
        ><label
          class="expression-search flex-1 flex items-center gap-2 h-8 pt-0 pr-1 pb-0 pl-2.5 min-w-0 bg-rail rounded-sm text-faint"
          ><AppIcon
            name="search"
            :size="15" /><input
            ref="search"
            v-model="query"
            :aria-label="
              tab === 'emoji'
                ? 'Search emoji'
                : gifView === 'favorites'
                  ? 'Search favorite GIFs'
                  : 'Search Klipy GIFs'
            "
            :placeholder="
              tab === 'emoji'
                ? 'Search emoji'
                : gifView === 'favorites'
                  ? 'Search favorites'
                  : 'Search Klipy'
            "
            maxlength="100" /><UiButton
            v-if="query"
            variant="ghost"
            size="icon-sm"
            aria-label="Clear search"
            @click="clearSearch"
          >
            <AppIcon
              name="close"
              :size="13" /></UiButton></label
        ><button
          v-if="tab === 'emoji'"
          class="skin-tone-button w-8 h-8 rounded-sm text-[20px]"
          aria-label="Choose skin tone"
          :aria-expanded="tonesOpen"
          @click="tonesOpen = !tonesOpen"
        >
          {{ tones[Number(library.library.skinTone)] }}
        </button>
      </header>
      <div
        v-if="tonesOpen && tab === 'emoji'"
        class="skin-tone-choices flex justify-end gap-0.5 pt-0 pr-2.5 pb-2 pl-2.5"
      >
        <button
          v-for="(tone, index) in tones"
          :key="tone"
          :aria-label="`Use ${toneNames[index]} skin tone`"
          :aria-pressed="library.library.skinTone === String(index)"
          @click="(library.setSkinTone(String(index)), (tonesOpen = false))"
        >
          {{ tone }}
        </button>
      </div>
      <div
        :id="bodyId"
        class="expression-body flex-1 min-h-0 flex"
        :role="emojiOnly ? undefined : 'tabpanel'"
        :aria-labelledby="emojiOnly ? undefined : tab === 'gifs' ? gifsId : emojiId"
        :aria-busy="tab === 'gifs' ? gifLoading : emojiLoading"
      >
        <template v-if="tab === 'gifs'">
          <div
            v-if="!media.gifsReady"
            class="picker-empty flex-1 flex flex-col items-center justify-center gap-1.5 p-6 text-center text-faint"
          >
            <AppIcon
              name="gif"
              :size="28"
            /><strong>GIF search isn’t set up</strong>
            <p>This server needs a Klipy app key.</p>
          </div>
          <div
            v-else-if="gifError && !gifItems.length"
            class="picker-empty flex-1 flex flex-col items-center justify-center gap-1.5 p-6 text-center text-faint"
            role="alert"
          >
            <AppIcon
              name="offline"
              :size="26"
            /><strong>Couldn’t load GIFs</strong>
            <p>{{ gifError.message }}</p>
            <UiButton
              variant="secondary"
              size="sm"
              @click="gifView === 'home' ? categories.refetch() : gifResults.refetch()"
            >
              Try again
            </UiButton>
          </div>
          <div
            v-else-if="gifLoading"
            class="gif-skeletons flex-1 grid [grid-template-columns:1fr_1fr] gap-1.5 pt-0 pr-2.5 pb-2.5 pl-2.5 [align-content:start]"
            aria-label="Loading GIFs"
          >
            <span
              v-for="i in 8"
              :key="i"
            />
          </div>
          <div
            v-else-if="gifView === 'home'"
            class="gif-scroll flex-1 min-h-0 overflow-y-auto pt-0 pr-2.5 pb-2.5 pl-2.5"
          >
            <div
              class="gif-category-grid grid [grid-template-columns:repeat(auto-fill,_minmax(140px,_1fr))] gap-1.5"
            >
              <button
                class="gif-category relative h-24 rounded-md overflow-hidden bg-secondary gif-collection"
                @click="openFavorites"
              >
                <NuxtImg
                  v-if="library.library.gifs[0]"
                  :src="library.library.gifs[0].stillUrl"
                  alt=""
                  referrerpolicy="no-referrer"
                /><span
                  ><AppIcon
                    name="star"
                    :size="16"
                  />Favorites</span
                >
              </button>
              <button
                class="gif-category relative h-24 rounded-md overflow-hidden bg-secondary gif-collection"
                @click="trending"
              >
                <NuxtImg
                  v-if="categories.data.value?.[0]"
                  :src="categories.data.value[0].previewUrl"
                  alt=""
                  referrerpolicy="no-referrer"
                /><span
                  ><AppIcon
                    name="trending"
                    :size="16"
                  />Trending</span
                >
              </button>
              <button
                v-for="item in categories.data.value"
                :key="item.query"
                class="gif-category relative h-24 rounded-md overflow-hidden bg-secondary"
                @click="category(item)"
              >
                <NuxtImg
                  :src="item.previewUrl"
                  alt=""
                  referrerpolicy="no-referrer"
                /><span>{{ item.name }}</span>
              </button>
            </div>
          </div>
          <div
            v-else
            class="gif-scroll flex-1 min-h-0 overflow-y-auto pt-0 pr-2.5 pb-2.5 pl-2.5"
            @scroll.passive="gifScroll"
          >
            <div
              class="picker-section-heading flex items-center gap-1.5 h-7 py-0 px-1 text-[11px] uppercase tracking-[0.05em] text-faint"
            >
              <strong>{{
                gifView === 'favorites'
                  ? 'Favorites'
                  : debouncedGifQuery && debouncedGifQuery !== categoryTitle
                    ? `“${debouncedGifQuery}”`
                    : categoryTitle
              }}</strong>
            </div>
            <div
              v-if="!gifItems.length"
              class="picker-empty flex-1 flex flex-col items-center justify-center gap-1.5 p-6 text-center text-faint"
            >
              <AppIcon
                :name="gifView === 'favorites' ? 'star' : 'search'"
                :size="26"
              /><strong>{{
                gifView === 'favorites' ? 'No favorites yet' : 'No GIFs found'
              }}</strong>
              <p>
                {{
                  gifView === 'favorites'
                    ? 'Star a GIF to keep it here on every device.'
                    : 'Try a different word.'
                }}
              </p>
            </div>
            <div
              v-else
              class="gif-masonry grid [grid-template-columns:1fr_1fr] gap-1.5 [align-items:start]"
            >
              <div
                v-for="(column, c) in gifColumns"
                :key="c"
                class="gif-column grid gap-1.5"
              >
                <div
                  v-for="item in column"
                  :key="item.id"
                  class="gif-tile relative rounded-md overflow-hidden bg-secondary"
                  :style="{ aspectRatio: `${item.width} / ${item.height}` }"
                >
                  <button
                    class="gif-select block w-full h-full"
                    :aria-label="`Send ${item.title} GIF`"
                    @click="emit('gif', item, debouncedGifQuery)"
                  >
                    <NuxtImg
                      :src="appearance.animate ? item.previewUrl : item.stillUrl"
                      :alt="item.title"
                      loading="lazy"
                      decoding="async"
                      referrerpolicy="no-referrer"
                    /></button
                  ><button
                    class="gif-save absolute top-1.5 right-1.5 w-7 h-7 flex items-center justify-center rounded-sm [background:rgb(15_15_20_/_72%)] text-[#fff] [opacity:0] [transition:opacity_0.12s]"
                    :class="{ 'is-favorite': library.isFavoriteGif(item) }"
                    :aria-label="
                      library.isFavoriteGif(item)
                        ? `Remove ${item.title} from favorites`
                        : `Save ${item.title} to favorites`
                    "
                    :aria-pressed="library.isFavoriteGif(item)"
                    @click="library.toggleGif(item)"
                  >
                    <AppIcon
                      name="star"
                      :size="15"
                    />
                  </button>
                </div>
              </div>
            </div>
            <p
              v-if="gifView === 'results' && gifResults.isLoading.value && gifItems.length"
              class="picker-more p-2.5 text-center text-faint text-[12px]"
            >
              Loading more…
            </p>
          </div>
        </template>
        <template v-else>
          <nav
            class="emoji-category-rail flex flex-col gap-0.5 pt-0 pr-1.5 pb-2 pl-1.5 overflow-y-auto [scrollbar-width:none] border-r [border-right-style:solid] border-r-border"
            aria-label="Emoji categories"
          >
            <button
              v-for="group in groups"
              :key="group.id"
              :aria-label="group.label"
              :title="group.label"
              :aria-pressed="emojiGroup === group.id && !emojiQuery"
              :class="{ active: emojiGroup === group.id && !emojiQuery }"
              @click="selectGroup(group.id)"
            >
              <AppIcon
                :name="group.icon"
                :size="18"
              />
            </button>
          </nav>
          <div class="emoji-content flex-1 min-w-0 flex flex-col pt-0 pr-1 pb-0 pl-2">
            <div
              class="picker-section-heading flex items-center gap-1.5 h-7 py-0 px-1 text-[11px] uppercase tracking-[0.05em] text-faint"
            >
              <strong>{{ emojiTitle }}</strong>
            </div>
            <div
              v-if="emojiLoading"
              class="emoji-loading grid [grid-template-columns:repeat(auto-fill,_minmax(36px,_1fr))] gap-1.5"
              aria-label="Loading emoji"
            >
              <span
                v-for="i in 40"
                :key="i"
              />
            </div>
            <div
              v-else-if="!displayedEmoji.length"
              class="picker-empty flex-1 flex flex-col items-center justify-center gap-1.5 p-6 text-center text-faint"
            >
              <AppIcon
                name="search"
                :size="24"
              /><strong>No emoji found</strong>
              <p>Try a name, feeling or :shortcode:.</p>
            </div>
            <EmojiGrid
              v-else
              :items="displayedEmoji"
              :skin-tone="library.library.skinTone"
              @choose="chooseEmoji"
              @preview="preview = $event"
            />
          </div>
        </template>
      </div>
      <footer
        v-if="tab === 'emoji' || gifView !== 'home'"
        class="expression-footer flex items-center gap-2.5 min-h-11.5 py-1.5 px-3 bg-sidebar border-t [border-top-style:solid] border-t-border text-[12px] text-faint shrink-0"
      >
        <template v-if="tab === 'gifs'"
          ><span>{{ gifView === 'favorites' ? `${library.library.gifs.length} saved` : '' }}</span
          ><a
            href="https://klipy.com"
            target="_blank"
            rel="noopener noreferrer"
            class="klipy-attribution ml-auto text-faint text-[11px]"
            >Powered by <strong>KLIPY</strong></a
          ></template
        ><template v-else-if="preview"
          ><span class="emoji-preview-glyph text-[26px] leading-[1]"
            ><span v-if="preview.unicode">{{ emojiText(preview, library.library.skinTone) }}</span
            ><NuxtImg
              v-else
              :src="preview.staticUrl || preview.url"
              alt="" /></span
          ><span class="emoji-preview-label flex flex-col min-w-0 flex-1"
            ><strong>:{{ preview.shortcode }}:</strong><small>{{ preview.label }}</small></span
          ><a
            v-if="preview.sourceUrl && (preview.source === '7tv' || preview.provider)"
            :href="preview.sourceUrl"
            target="_blank"
            rel="noopener noreferrer"
            :title="preview.creator ? `By ${preview.creator}` : undefined"
            >{{ preview.source === '7tv' ? '7TV' : providerName(preview.provider) }}</a
          ></template
        ><span
          v-else
          class="emoji-hint text-faint"
          >Hover an emoji to preview it</span
        >
      </footer>
    </div>
  </Teleport>
</template>
