<template>
  <div v-if="isOpen" class="gif-picker" @click.self="emit('close')">
    <div class="panel" role="dialog" aria-label="GIF picker" @keydown.esc.prevent="emit('close')">
      <div class="panel-header">
        <div class="title-wrap">
          <div class="title">GIFs</div>
          <div class="hint">{{ shortcutHint }}</div>
        </div>
        <button type="button" class="close-btn" @click="emit('close')">x</button>
      </div>

      <div class="search-row">
        <input
          ref="searchEl"
          v-model="query"
          type="text"
          class="search"
          placeholder="Search GIFs"
        >
      </div>

      <div class="meta-row">
        <span class="badge">{{ modeLabel }}</span>
        <span class="meta">{{ gifs.length }} results</span>
      </div>

      <div class="categories" role="group" aria-label="GIF categories">
        <button
          v-for="category in gifCategories"
          :key="category.id"
          type="button"
          class="category"
          :class="{ active: activeCategoryId === category.id }"
          @click="setCategory(category.id)"
        >
          {{ category.label }}
        </button>
      </div>

      <div class="grid-wrap">
        <div v-if="errorText" class="status-line">{{ errorText }}</div>
        <div v-else-if="isLoading && !gifs.length" class="status-line">Loading GIFs...</div>
        <div v-else-if="!gifs.length" class="status-line">No GIFs found.</div>

        <div class="grid" role="listbox" aria-label="GIF search results">
          <button
            v-for="gif in gifs"
            :key="gif.id"
            type="button"
            class="tile"
            :title="gif.title || 'GIF'"
            @click="selectGif(gif)"
          >
            <img :src="gif.previewUrl || gif.url" :alt="gif.title || 'GIF'" loading="lazy">
            <span class="overlay">Send GIF</span>
          </button>
        </div>
      </div>

      <div class="panel-footer">
        <button
          type="button"
          class="load-more"
          :disabled="isLoading"
          @click="loadNextPage"
        >
          {{ isLoading ? 'Loading...' : 'Load More' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
interface Props {
  isOpen: boolean
}

interface GifItem {
  id: string
  title: string
  url: string
  playbackUrl: string
  previewUrl: string
}

interface GifCategory {
  id: string
  label: string
  query: string
}

const props = defineProps<Props>()

const emit = defineEmits<{
  close: []
  select: [url: string]
}>()

const API_BASE = 'https://api.klipy.com/api/v1'
const API_KEY = '9CKW7ub0jWCyDC4TGI7IQKsE8TeUom0NzpflIfQuljAqx7WnayXzlWbBMOPYaAOx'
const PAGE_SIZE = 24

const gifCategories: GifCategory[] = [
  { id: 'trending', label: 'Trending', query: '' },
  { id: 'reactions', label: 'Reactions', query: 'reaction' },
  { id: 'lol', label: 'LOL', query: 'funny' },
  { id: 'hype', label: 'Hype', query: 'hype' },
  { id: 'gaming', label: 'Gaming', query: 'gaming' },
  { id: 'anime', label: 'Anime', query: 'anime' },
  { id: 'cats', label: 'Cats', query: 'cat' },
]

const fallbackGifs: GifItem[] = [
  {
    id: 'fallback-1',
    title: 'Hi',
    url: 'https://media4.giphy.com/media/v1.Y2lkPTc5MGI3NjExNzE1M3R0MW45dXpmY2N2emhzN3VzY2NuYnphb2cwZjVrcDE5Nnc5biZlcD12MV9naWZzX3NlYXJjaCZjdD1n/QvBoMEcQ7DQXK/giphy.gif',
    playbackUrl: 'https://media4.giphy.com/media/v1.Y2lkPTc5MGI3NjExNzE1M3R0MW45dXpmY2N2emhzN3VzY2NuYnphb2cwZjVrcDE5Nnc5biZlcD12MV9naWZzX3NlYXJjaCZjdD1n/QvBoMEcQ7DQXK/giphy.mp4',
    previewUrl: 'https://media4.giphy.com/media/v1.Y2lkPTc5MGI3NjExNzE1M3R0MW45dXpmY2N2emhzN3VzY2NuYnphb2cwZjVrcDE5Nnc5biZlcD12MV9naWZzX3NlYXJjaCZjdD1n/QvBoMEcQ7DQXK/giphy.gif',
  },
  {
    id: 'fallback-2',
    title: 'Happy',
    url: 'https://media3.giphy.com/media/v1.Y2lkPTc5MGI3NjExN2g4cmNwMm9hMDR4Nnh3ZWptcnY1c3NneWM2aDR0Y3lyYWMwaW9jNiZlcD12MV9naWZzX3NlYXJjaCZjdD1n/11sBLVxNs7v6WA/giphy.gif',
    playbackUrl: 'https://media3.giphy.com/media/v1.Y2lkPTc5MGI3NjExN2g4cmNwMm9hMDR4Nnh3ZWptcnY1c3NneWM2aDR0Y3lyYWMwaW9jNiZlcD12MV9naWZzX3NlYXJjaCZjdD1n/11sBLVxNs7v6WA/giphy.mp4',
    previewUrl: 'https://media3.giphy.com/media/v1.Y2lkPTc5MGI3NjExN2g4cmNwMm9hMDR4Nnh3ZWptcnY1c3NneWM2aDR0Y3lyYWMwaW9jNiZlcD12MV9naWZzX3NlYXJjaCZjdD1n/11sBLVxNs7v6WA/giphy.gif',
  },
  {
    id: 'fallback-3',
    title: 'Party',
    url: 'https://media4.giphy.com/media/v1.Y2lkPTc5MGI3NjExYTZud3ZwaWR4NmF0MHRnYnF4a2YydW9ieXNta3NobmI4Z2F5MnNzaCZlcD12MV9naWZzX3NlYXJjaCZjdD1n/l4FGpP4lxGGgK5CBW/giphy.gif',
    playbackUrl: 'https://media4.giphy.com/media/v1.Y2lkPTc5MGI3NjExYTZud3ZwaWR4NmF0MHRnYnF4a2YydW9ieXNta3NobmI4Z2F5MnNzaCZlcD12MV9naWZzX3NlYXJjaCZjdD1n/l4FGpP4lxGGgK5CBW/giphy.mp4',
    previewUrl: 'https://media4.giphy.com/media/v1.Y2lkPTc5MGI3NjExYTZud3ZwaWR4NmF0MHRnYnF4a2YydW9ieXNta3NobmI4Z2F5MnNzaCZlcD12MV9naWZzX3NlYXJjaCZjdD1n/l4FGpP4lxGGgK5CBW/giphy.gif',
  },
]

const query = ref('')
const gifs = ref<GifItem[]>([])
const currentPage = ref(1)
const isLoading = ref(false)
const errorText = ref('')
const debounceHandle = ref<ReturnType<typeof setTimeout> | null>(null)
const searchEl = ref<HTMLInputElement>()
const activeCategoryId = ref('trending')

const shortcutHint = computed(() => {
  if (!import.meta.client) return 'Ctrl+G'
  const ua = navigator.userAgent.toLowerCase()
  const isApple = ua.includes('mac') || ua.includes('iphone') || ua.includes('ipad')
  return isApple ? 'Cmd+G' : 'Ctrl+G'
})

const modeLabel = computed(() => (query.value.trim().length >= 2 ? 'Search' : 'Trending'))

const locale = computed(() => {
  if (!import.meta.client) return 'us'
  const region = navigator.language.split('-')[1]
  return (region || 'us').toLowerCase()
})

const customerId = computed(() => {
  if (!import.meta.client) return 'mock-user'
  const storageKey = 'ircord-klipy-customer-id'
  const existing = localStorage.getItem(storageKey)
  if (existing) return existing

  const generated = `mock-${Math.random().toString(36).slice(2, 10)}`
  localStorage.setItem(storageKey, generated)
  return generated
})

function extractUrl(item: unknown): string {
  const candidate = item as {
    file?: {
      sm?: { gif?: { url?: string }, webp?: { url?: string } },
      md?: { gif?: { url?: string }, webp?: { url?: string } },
      hd?: { gif?: { url?: string } }
    }
  }

  return (
    candidate.file?.sm?.gif?.url
    || candidate.file?.md?.gif?.url
    || candidate.file?.hd?.gif?.url
    || candidate.file?.sm?.webp?.url
    || candidate.file?.md?.webp?.url
    || ''
  )
}

function extractPreviewUrl(item: unknown): string {
  const candidate = item as {
    file?: {
      sm?: { gif?: { url?: string }, webp?: { url?: string } },
      md?: { gif?: { url?: string }, webp?: { url?: string } }
    }
  }

  return (
    candidate.file?.sm?.webp?.url
    || candidate.file?.sm?.gif?.url
    || candidate.file?.md?.webp?.url
    || candidate.file?.md?.gif?.url
    || extractUrl(item)
  )
}

function extractVideoUrl(item: unknown): string {
  const candidate = item as {
    file?: {
      sm?: { mp4?: { url?: string }, webm?: { url?: string }, gifv?: { url?: string } },
      md?: { mp4?: { url?: string }, webm?: { url?: string }, gifv?: { url?: string } },
      hd?: { mp4?: { url?: string }, webm?: { url?: string }, gifv?: { url?: string } }
    }
  }

  return (
    candidate.file?.sm?.mp4?.url
    || candidate.file?.md?.mp4?.url
    || candidate.file?.hd?.mp4?.url
    || candidate.file?.sm?.webm?.url
    || candidate.file?.md?.webm?.url
    || candidate.file?.hd?.webm?.url
    || candidate.file?.sm?.gifv?.url
    || candidate.file?.md?.gifv?.url
    || candidate.file?.hd?.gifv?.url
    || ''
  )
}

function deriveVideoFromGif(url: string): string {
  if (/giphy\.gif(?:\?.*)?$/i.test(url)) {
    return url.replace(/giphy\.gif(?:\?.*)?$/i, 'giphy.mp4')
  }
  if (/media\.tenor\.com/i.test(url) && /\.gif(?:\?.*)?$/i.test(url)) {
    return url.replace(/\.gif(?:\?.*)?$/i, '.mp4')
  }
  if (/\.gifv(?:\?.*)?$/i.test(url)) {
    return url.replace(/\.gifv(?:\?.*)?$/i, '.mp4')
  }
  return ''
}

function normalizeItems(payload: unknown): GifItem[] {
  const root = payload as { data?: { data?: unknown[] } | unknown[] }
  const items = Array.isArray(root.data)
    ? root.data
    : Array.isArray(root.data?.data)
      ? root.data.data
      : []

  return items
    .map((item): GifItem | null => {
      const resolved = item as { id?: string | number, title?: string, slug?: string }
      const url = extractUrl(item)
      if (!url) return null

      return {
        id: String(resolved.id ?? Math.random().toString(36).slice(2, 9)),
        title: resolved.title || resolved.slug || 'GIF',
        url,
        playbackUrl: extractVideoUrl(item) || deriveVideoFromGif(url) || url,
        previewUrl: extractPreviewUrl(item),
      }
    })
    .filter((item): item is GifItem => Boolean(item))
}

async function fetchGifs(page: number, append = false) {
  isLoading.value = true
  errorText.value = ''

  const endpoint = query.value.trim().length >= 2 ? 'search' : 'trending'
  const params = new URLSearchParams({
    page: String(page),
    per_page: String(PAGE_SIZE),
    customer_id: customerId.value,
    locale: locale.value,
    content_filter: 'medium',
    format_filter: 'gif,webp,mp4',
  })

  if (endpoint === 'search') {
    params.set('q', query.value.trim())
  }

  try {
    const response = await fetch(`${API_BASE}/${API_KEY}/gifs/${endpoint}?${params.toString()}`)
    if (!response.ok) {
      throw new Error(`GIF API request failed with status ${response.status}`)
    }

    const payload = await response.json()
    const normalized = normalizeItems(payload)

    gifs.value = append ? [...gifs.value, ...normalized] : normalized
  } catch {
    errorText.value = 'Klipy API unavailable. Showing fallback GIFs.'
    gifs.value = append ? [...gifs.value, ...fallbackGifs] : [...fallbackGifs]
  } finally {
    isLoading.value = false
  }
}

function selectGif(gif: GifItem) {
  emit('select', gif.playbackUrl || gif.url)
}

function setCategory(categoryId: string) {
  const category = gifCategories.find((item) => item.id === categoryId)
  if (!category) return

  activeCategoryId.value = categoryId
  query.value = category.query
  currentPage.value = 1
  fetchGifs(1)
}

function loadNextPage() {
  currentPage.value += 1
  fetchGifs(currentPage.value, true)
}

watch(
  () => props.isOpen,
  async (open) => {
    if (!open) return
    currentPage.value = 1
    await fetchGifs(1)
    await nextTick()
    searchEl.value?.focus()
    searchEl.value?.select()
  }
)

watch(query, () => {
  if (query.value.trim().length === 0) {
    activeCategoryId.value = 'trending'
  }

  if (debounceHandle.value) {
    clearTimeout(debounceHandle.value)
  }

  debounceHandle.value = setTimeout(() => {
    currentPage.value = 1
    fetchGifs(1)
  }, 260)
})

onBeforeUnmount(() => {
  if (debounceHandle.value) {
    clearTimeout(debounceHandle.value)
  }
})
</script>

<style scoped>
.gif-picker {
  position: absolute;
  width: min(570px, calc(100vw - 32px));
  right: 16px;
  bottom: 58px;
  z-index: 30;
}

.panel {
  max-height: min(78vh, 780px);
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: linear-gradient(180deg, rgba(44, 47, 55, 0.98), rgba(39, 42, 49, 0.98));
  box-shadow: 0 20px 48px rgba(0, 0, 0, 0.45);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.title-wrap {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.title {
  font-family: var(--font-mono);
  font-size: 21px;
  color: var(--text-primary);
  font-weight: 700;
}

.hint {
  font-family: var(--font-mono);
  font-size: 16px;
  color: var(--text-faint);
}

.search-row {
  padding: 10px;
}

.search {
  width: 100%;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 10px;
  background-color: var(--bg-input);
  color: var(--text-body);
  padding: 12px 15px;
  font-family: var(--font-mono);
  font-size: 19px;
  outline: none;
}

.search:focus {
  border-color: var(--accent-blurple);
}

.meta-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 10px 12px 10px;
}

.badge {
  font-family: var(--font-mono);
  font-size: 16px;
  color: #d8defe;
  background-color: rgba(88, 101, 242, 0.24);
  border-radius: 999px;
  padding: 4px 12px;
}

.meta {
  font-family: var(--font-mono);
  font-size: 16px;
  color: var(--text-muted);
}

.categories {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 0 10px 12px 10px;
}

.category {
  border: 0;
  border-radius: 999px;
  background-color: rgba(255, 255, 255, 0.05);
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: 13px;
  white-space: normal;
  padding: 7px 11px;
  cursor: pointer;
}

.category.active {
  background-color: rgba(88, 101, 242, 0.2);
  color: var(--text-primary);
}

.close-btn,
.load-more {
  border: 0;
  border-radius: 10px;
  background-color: var(--bg-input);
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: 18px;
  padding: 12px 15px;
  cursor: pointer;
}

.close-btn:hover,
.load-more:hover {
  background-color: var(--bg-active);
}

.close-btn:disabled,
.load-more:disabled {
  opacity: 0.6;
  cursor: default;
}

.grid-wrap {
  max-height: min(54vh, 540px);
  overflow-y: auto;
}

.status-line {
  padding: 10px;
  font-family: var(--font-mono);
  font-size: 17px;
  color: var(--text-muted);
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(156px, 1fr));
  gap: 12px;
  padding: 12px;
}

.tile {
  position: relative;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 10px;
  padding: 0;
  overflow: hidden;
  background: transparent;
  cursor: pointer;
}

.tile:hover {
  border-color: rgba(88, 101, 242, 0.5);
}

.tile img {
  width: 100%;
  height: 162px;
  object-fit: cover;
  display: block;
}

.overlay {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 39px;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0), rgba(0, 0, 0, 0.75));
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: var(--font-mono);
  font-size: 14px;
  color: rgba(255, 255, 255, 0.95);
  opacity: 0;
  transition: opacity 0.12s ease;
}

.tile:hover .overlay,
.tile:focus-visible .overlay {
  opacity: 1;
}

.panel-footer {
  padding: 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  justify-content: center;
}

@media (max-width: 900px) {
  .gif-picker {
    width: calc(100vw - 16px);
    right: 8px;
  }
}
</style>
