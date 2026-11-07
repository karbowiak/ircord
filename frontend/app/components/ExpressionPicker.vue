<script setup lang="ts">
import { useInfiniteQuery, useQuery } from '@pinia/colada'
import type { EmojiItem, GifCategory, GifItem } from '../../shared/types/media'
import { emojiText, rankEmoji, searchEmoji } from '../../shared/utils/media'
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
  sheet.value = window.innerWidth <= 700
  if (sheet.value) {
    placedBelow.value = false
    const height = Math.round(Math.min(viewportHeight * 0.62, 520))
    position.value = {
      left: '0px',
      top: `${viewportHeight - height}px`,
      width: '100vw',
      height: `${height}px`,
    }
    return
  }
  const width = Math.min(layout.state.pickerWidth, window.innerWidth - 16)
  const above = rect.top - 16
  const below = viewportHeight - rect.bottom - 16
  placedBelow.value = below > above && above < 360
  const height = Math.max(
    240,
    Math.min(layout.state.pickerHeight, placedBelow.value ? below : above),
  )
  position.value = {
    left: `${Math.max(8, Math.min(rect.right - width, window.innerWidth - width - 8))}px`,
    top: `${placedBelow.value ? rect.bottom + 8 : Math.max(8, rect.top - height - 8)}px`,
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
  search.value?.focus()
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
      class="expression-picker"
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
        class="picker-resize"
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
      <header class="expression-header">
        <div
          v-if="!emojiOnly"
          class="expression-tabs"
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
        <button
          v-if="tab === 'gifs' && gifView !== 'home'"
          class="icon-button"
          aria-label="Back to GIF categories"
          @click="back"
        >
          <AppIcon
            name="back"
            :size="16"
          /></button
        ><label class="expression-search"
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
            maxlength="100" /><button
            v-if="query"
            class="icon-button"
            aria-label="Clear search"
            @click="clearSearch"
          >
            <AppIcon
              name="close"
              :size="13"
            /></button></label
        ><button
          v-if="tab === 'emoji'"
          class="skin-tone-button"
          aria-label="Choose skin tone"
          :aria-expanded="tonesOpen"
          @click="tonesOpen = !tonesOpen"
        >
          {{ tones[Number(library.library.skinTone)] }}
        </button>
      </header>
      <div
        v-if="tonesOpen && tab === 'emoji'"
        class="skin-tone-choices"
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
        class="expression-body"
        :role="emojiOnly ? undefined : 'tabpanel'"
        :aria-labelledby="emojiOnly ? undefined : tab === 'gifs' ? gifsId : emojiId"
        :aria-busy="tab === 'gifs' ? gifLoading : emojiLoading"
      >
        <template v-if="tab === 'gifs'">
          <div
            v-if="!media.gifsReady"
            class="picker-empty"
          >
            <AppIcon
              name="gif"
              :size="28"
            /><strong>GIF search isn’t set up</strong>
            <p>This server needs a Klipy app key.</p>
          </div>
          <div
            v-else-if="gifError && !gifItems.length"
            class="picker-empty"
            role="alert"
          >
            <AppIcon
              name="offline"
              :size="26"
            /><strong>Couldn’t load GIFs</strong>
            <p>{{ gifError.message }}</p>
            <button
              class="button secondary small"
              @click="gifView === 'home' ? categories.refetch() : gifResults.refetch()"
            >
              Try again
            </button>
          </div>
          <div
            v-else-if="gifLoading"
            class="gif-skeletons"
            aria-label="Loading GIFs"
          >
            <span
              v-for="i in 8"
              :key="i"
            />
          </div>
          <div
            v-else-if="gifView === 'home'"
            class="gif-scroll"
          >
            <div class="gif-category-grid">
              <button
                class="gif-category gif-collection"
                @click="openFavorites"
              >
                <img
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
                class="gif-category gif-collection"
                @click="trending"
              >
                <img
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
                class="gif-category"
                @click="category(item)"
              >
                <img
                  :src="item.previewUrl"
                  alt=""
                  referrerpolicy="no-referrer"
                /><span>{{ item.name }}</span>
              </button>
            </div>
          </div>
          <div
            v-else
            class="gif-scroll"
            @scroll.passive="gifScroll"
          >
            <div class="picker-section-heading">
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
              class="picker-empty"
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
              class="gif-masonry"
            >
              <div
                v-for="(column, c) in gifColumns"
                :key="c"
                class="gif-column"
              >
                <div
                  v-for="item in column"
                  :key="item.id"
                  class="gif-tile"
                  :style="{ aspectRatio: `${item.width} / ${item.height}` }"
                >
                  <button
                    class="gif-select"
                    :aria-label="`Send ${item.title} GIF`"
                    @click="emit('gif', item, debouncedGifQuery)"
                  >
                    <img
                      :src="appearance.animate ? item.previewUrl : item.stillUrl"
                      :alt="item.title"
                      loading="lazy"
                      decoding="async"
                      referrerpolicy="no-referrer"
                    /></button
                  ><button
                    class="gif-save"
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
              class="picker-more"
            >
              Loading more…
            </p>
          </div>
        </template>
        <template v-else>
          <nav
            class="emoji-category-rail"
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
          <div class="emoji-content">
            <div class="picker-section-heading">
              <strong>{{ emojiTitle }}</strong>
            </div>
            <div
              v-if="emojiLoading"
              class="emoji-loading"
              aria-label="Loading emoji"
            >
              <span
                v-for="i in 40"
                :key="i"
              />
            </div>
            <div
              v-else-if="!displayedEmoji.length"
              class="picker-empty"
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
        class="expression-footer"
      >
        <template v-if="tab === 'gifs'"
          ><span>{{ gifView === 'favorites' ? `${library.library.gifs.length} saved` : '' }}</span
          ><a
            href="https://klipy.com"
            target="_blank"
            rel="noopener noreferrer"
            class="klipy-attribution"
            >Powered by <strong>KLIPY</strong></a
          ></template
        ><template v-else-if="preview"
          ><span class="emoji-preview-glyph"
            ><span v-if="preview.unicode">{{ emojiText(preview, library.library.skinTone) }}</span
            ><img
              v-else
              :src="preview.staticUrl || preview.url"
              alt="" /></span
          ><span class="emoji-preview-label"
            ><strong>:{{ preview.shortcode }}:</strong><small>{{ preview.label }}</small></span
          ><a
            v-if="preview.source === '7tv'"
            :href="preview.sourceUrl"
            target="_blank"
            rel="noopener noreferrer"
            >7TV</a
          ></template
        ><span
          v-else
          class="emoji-hint"
          >Hover an emoji to preview it</span
        >
      </footer>
    </div>
  </Teleport>
</template>
