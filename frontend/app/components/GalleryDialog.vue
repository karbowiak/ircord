<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { GalleryItem, ViewerItem } from '../../shared/types/chat'
import { galleryLoads } from '../../shared/utils/gallery'
import { GALLERY_PAGE, galleryQuery, galleryRevision } from '../queries/gallery'

/**
 * Every image and GIF posted in one conversation, newest post first, from your stored history.
 * Images load lazily and only where the timeline would load them; the rest stay links. Opening
 * one shows it in the image viewer, which pages through the loaded gallery; “Jump to message”
 * opens the post in the conversation.
 */
const props = defineProps<{ conversationId: string }>()
const emit = defineEmits<{ close: []; open: [item: GalleryItem] }>()
const chat = useChatStore()
const preferences = usePreferencesStore()
const { full, time } = useTimeFormat()
/** Today's posts by time, older ones by date, so a tile caption stays short. */
const when = (iso: string) =>
  new Date(iso).toDateString() === new Date().toDateString()
    ? time(iso)
    : new Date(iso).toLocaleDateString(undefined, {
        month: 'short',
        day: 'numeric',
        year: '2-digit',
      })

const conversation = computed(() =>
  chat.data?.conversations.find((c) => c.id === props.conversationId),
)
const title = computed(() => {
  const c = conversation.value
  if (!c) return 'Gallery'
  return c.kind === 'channel'
    ? `Gallery · #${c.name}`
    : `Gallery · ${chat.directPartner(c)?.nick ?? c.name}`
})
const network = computed(
  () => chat.data?.networks.find((n) => n.id === conversation.value?.networkId)?.name ?? '',
)

const first = useQuery(() => galleryQuery(props.conversationId))
/** Pages after the first, loaded as the grid scrolls; dropped whenever the gallery changed. */
const older = ref<GalleryItem[]>([])
const olderCursor = ref<string | null>(null)
const loadingMore = ref(false)
const moreError = ref('')
watch(galleryRevision, () => {
  older.value = []
  olderCursor.value = null
  moreError.value = ''
})
const cursor = computed(() =>
  older.value.length ? olderCursor.value : (first.data.value?.nextCursor ?? null),
)
const key = (item: GalleryItem) => `${item.messageId}:${item.index}`
async function loadMore() {
  if (!cursor.value || loadingMore.value) return
  loadingMore.value = true
  moreError.value = ''
  const revision = galleryRevision.value
  try {
    const page = await apiClient().gallery(props.conversationId, {
      cursor: cursor.value,
      limit: GALLERY_PAGE,
    })
    if (revision !== galleryRevision.value) return
    const known = new Set([...(first.data.value?.items ?? []), ...older.value].map(key))
    older.value = [...older.value, ...page.items.filter((i) => !known.has(key(i)))]
    olderCursor.value = page.nextCursor ?? null
  } catch (error) {
    moreError.value = errorText(error)
  } finally {
    loadingMore.value = false
  }
}
const items = computed(() => [...(first.data.value?.items ?? []), ...older.value])
const total = computed(() => first.data.value?.total ?? 0)
const loading = computed(() => first.isLoading.value && !first.data.value)

/** Scrolling near the end of the grid loads the next page. */
const sentinel = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | undefined
onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => {
      if (entries.some((e) => e.isIntersecting) && !moreError.value) void loadMore()
    },
    { rootMargin: '400px' },
  )
})
watch(sentinel, (el, previous) => {
  if (previous) observer?.unobserve(previous)
  if (el) observer?.observe(el)
})
onBeforeUnmount(() => observer?.disconnect())

const channelHosts = computed(() => conversation.value?.channelSettings?.trustedMediaHosts ?? [])
const loads = (item: GalleryItem) =>
  galleryLoads(item, preferences.preferences.linkPreviews, location.origin, channelHosts.value)
const host = (item: GalleryItem) => {
  try {
    return new URL(item.url, location.href).hostname.replace(/^www\./, '')
  } catch {
    return item.url
  }
}
const author = (item: GalleryItem) =>
  chat.member(item.senderId)?.displayName ?? chat.member(item.senderId)?.nick ?? 'Someone'
const label = (item: GalleryItem) =>
  `${item.kind === 'gif' ? 'GIF' : 'Image'} from ${author(item)}, ${full(item.createdAt)}`

/** Images that failed to load; their tile becomes a link to the original. */
const failed = reactive(new Set<string>())
const shown = computed(() => items.value.filter((i) => loads(i) && !failed.has(i.url)))
const viewer = ref<{ items: ViewerItem[]; index: number } | null>(null)
function view(item: GalleryItem) {
  const list = shown.value
  viewer.value = {
    items: list.map((i) => ({ url: i.url, name: `${author(i)} · ${full(i.createdAt)}` })),
    index: Math.max(
      0,
      list.findIndex((i) => key(i) === key(item)),
    ),
  }
}
</script>
<template>
  <AppDialog
    :title="title"
    wide
    @close="emit('close')"
  >
    <div class="gallery-view-summary">
      <span v-if="first.data.value"
        >{{ total === 1 ? '1 image' : `${total} images`
        }}<template v-if="network"> · {{ network }}</template></span
      >
      <span
        v-if="first.data.value?.indexing"
        class="gallery-view-note"
        role="status"
        >Older history is still being indexed, so some older images may be missing.</span
      >
    </div>
    <div class="gallery-view-scroll">
      <p
        v-if="loading"
        class="search-empty"
      >
        Loading images…
      </p>
      <div
        v-else-if="first.error.value && !first.data.value"
        class="search-empty"
        role="alert"
      >
        <AppIcon
          name="alert"
          :size="27"
        />
        <p>Couldn’t load the gallery: {{ errorText(first.error.value) }}</p>
        <button
          class="text-button"
          @click="first.refetch()"
        >
          Try again
        </button>
      </div>
      <div
        v-else-if="!items.length"
        class="search-empty"
      >
        <AppIcon
          name="image"
          :size="28"
        />
        <p>
          No images here yet. Uploads, image links and GIFs posted in this conversation appear here,
          newest first.
        </p>
      </div>
      <ul
        v-else
        class="gallery-view-grid"
        aria-label="Images"
      >
        <li
          v-for="item in items"
          :key="key(item)"
          class="gallery-view-tile"
        >
          <MessageImage
            v-if="loads(item) && !failed.has(item.url)"
            class="gallery-view-image"
            :src="item.url"
            :label="`View ${label(item)}`"
            :animated="item.kind === 'gif'"
            external
            @open="view(item)"
            @error="failed.add(item.url)"
          />
          <a
            v-else
            class="gallery-view-link"
            :href="item.url"
            target="_blank"
            rel="noopener noreferrer"
            :title="item.url"
            ><AppIcon
              name="image"
              :size="22"
            /><span>{{ host(item) }}</span
            ><small>{{
              failed.has(item.url) ? 'Image unavailable' : 'Not previewed · open link'
            }}</small></a
          >
          <div class="gallery-view-caption">
            <span class="gallery-view-source"
              ><strong>{{ author(item) }}</strong
              ><time
                :datetime="item.createdAt"
                :title="full(item.createdAt)"
                >{{ when(item.createdAt) }}</time
              ></span
            >
            <button
              class="icon-button gallery-view-jump"
              :aria-label="`Jump to the message: ${item.excerpt || label(item)}`"
              :title="item.excerpt ? `Jump to message: ${item.excerpt}` : 'Jump to message'"
              @click="emit('open', item)"
            >
              <AppIcon
                name="message"
                :size="15"
              />
            </button>
          </div>
        </li>
      </ul>
      <p
        v-if="moreError"
        class="form-error"
        role="alert"
      >
        Couldn’t load more: {{ moreError }}
        <button
          class="text-button"
          @click="loadMore"
        >
          Try again
        </button>
      </p>
      <div
        v-if="cursor && !moreError"
        ref="sentinel"
        class="gallery-view-more"
      >
        <button
          class="text-button"
          :disabled="loadingMore"
          @click="loadMore"
        >
          {{ loadingMore ? 'Loading…' : 'Load older images' }}
        </button>
      </div>
    </div>
    <ImageViewer
      v-if="viewer"
      :items="viewer.items"
      :start="viewer.index"
      @close="viewer = null"
    />
  </AppDialog>
</template>
