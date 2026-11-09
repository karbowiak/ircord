<script setup lang="ts">
import { useQuery } from '@pinia/colada'
import type { LinkCard, LinkKind, LinkSource } from '../../shared/types/chat'
import { linkKinds, linkTitle, openableLink } from '../../shared/utils/link-catalog'
import { publicMediaURL } from '../../shared/utils/link-media'
import { linkMediaQuery } from '../queries/link-media'

/**
 * One resource in Links, as a card: a preview resolved from its newest post (the same request
 * and cache the timeline uses), what the page is, and who posted it where. The preview box has a
 * fixed shape, so cards never change size as previews arrive. Photos and videos open in the
 * media viewer, players play in place, and pages open in a new tab.
 */
const props = defineProps<{ card: LinkCard }>()
const emit = defineEmits<{ jump: [source: LinkSource] }>()
const chat = useChatStore()
const auth = useAuthStore()
const { full, day, time } = useTimeFormat()
const when = (iso: string) => `${day(iso)} ${time(iso)}`

const conversation = (id: string) => chat.data?.conversations.find((c) => c.id === id)
const networkName = (id: string) => chat.data?.networks.find((n) => n.id === id)?.name ?? 'Network'
const place = (s: LinkSource) => {
  const c = conversation(s.conversationId)
  return c ? `#${c.name}` : 'a closed channel'
}
const author = (s: LinkSource) =>
  chat.member(s.senderId)?.displayName || chat.member(s.senderId)?.nick || 'Someone'
const kindIcon = (k: LinkKind) => linkKinds.find((x) => x.value === k)?.icon ?? 'link'
const kindBadge: Partial<Record<LinkKind, string>> = {
  gif: 'GIF',
  video: 'Video',
  audio: 'Audio',
  youtube: 'YouTube',
}
const latest = computed(() => props.card.sources[0])
/** What the post said besides the link itself. */
const remark = computed(() => {
  const text = latest.value?.excerpt ?? ''
  return text
    .replace(/\bhttps?:\/\/\S+/gi, '')
    .replace(/[\s:–—-]+$/, '')
    .trim()
})

// Previews resolve once a card comes within a screen of the list's visible area.
const root = ref<HTMLElement | null>(null)
const nearby = ref(false)
let observer: IntersectionObserver | undefined
onMounted(() => {
  if (!('IntersectionObserver' in window) || !root.value) {
    nearby.value = true
    return
  }
  observer = new IntersectionObserver(([entry]) => (nearby.value = !!entry?.isIntersecting), {
    root: root.value.closest('.saved-results'),
    rootMargin: '100% 0px',
  })
  observer.observe(root.value)
})
onBeforeUnmount(() => observer?.disconnect())
const media = useQuery(() =>
  linkMediaQuery({
    account: auth.account?.id ?? '',
    conversation: latest.value?.conversationId ?? '',
    message: latest.value?.messageId ?? '',
    url: props.card.url,
    widget: false,
    enabled: nearby.value && !!latest.value,
  }),
)
const content = computed(() => media.data.value)
const assets = computed(() =>
  (content.value?.assets ?? [])
    .filter((a) => a.kind !== 'audio')
    .map((a) => ({ ...a, url: publicMediaURL(a.url) }))
    .filter((a) => a.url),
)
const asset = computed(() => assets.value[0])
const still = computed(() => {
  const url = content.value?.card?.image || content.value?.widget?.thumbnail
  return url ? publicMediaURL(url) : ''
})
const icon = computed(() => content.value?.card?.icon && publicMediaURL(content.value.card.icon))
const title = computed(
  () => content.value?.card?.title || content.value?.title || linkTitle(props.card),
)
const description = computed(() => content.value?.card?.description || content.value?.description)
const site = computed(() => content.value?.card?.site || props.card.host)
const loading = computed(() => nearby.value && media.isLoading.value && !content.value)
const previewFailed = ref(false)
const iconFailed = ref(false)
watch(
  () => props.card.url,
  () => {
    previewFailed.value = iconFailed.value = playing.value = false
  },
)

/** The viewer for photos and videos; a player widget plays in the card itself. */
const viewing = ref(false)
const playing = ref(false)
const playable = computed(() => !!content.value?.widget && !asset.value)
const href = computed(() => (openableLink(props.card.url) ? props.card.url : undefined))

/** GIFs hold a still frame until the card is hovered or keyboard-focused, as in the timeline. */
const appearance = useAppearanceStore()
const isGif = (url: string, type?: string) => {
  if (type === 'image/gif') return true
  try {
    return /\.gif$/i.test(new URL(url).pathname)
  } catch {
    return false
  }
}
const gif = computed(
  () =>
    !!asset.value &&
    asset.value.kind === 'image' &&
    isGif(asset.value.url, asset.value.contentType),
)
const stillGif = computed(() => !!still.value && isGif(still.value))
const hovered = ref(false)
const focused = ref(false)
const animating = computed(
  () => appearance.animate && nearby.value && (hovered.value || focused.value),
)
function enter(event: PointerEvent) {
  if (event.pointerType !== 'touch') hovered.value = true
}
function focusIn(event: FocusEvent) {
  focused.value = (event.target as Element).matches(':focus-visible')
}
function focusOut(event: FocusEvent) {
  if (!root.value?.contains(event.relatedTarget as Node | null)) focused.value = false
}

async function copy(url: string) {
  try {
    await navigator.clipboard.writeText(url)
    chat.notify('Link copied.')
  } catch {
    chat.notify('Couldn’t copy the link. Your browser blocked clipboard access.')
  }
}
</script>
<template>
  <li
    ref="root"
    class="catalog-card group relative flex min-w-0 flex-col overflow-hidden rounded-lg border border-solid border-border/60 bg-card transition-[border-color,box-shadow] hover:border-border focus-within:border-border [&.is-selected]:border-primary [&.is-selected]:shadow-[0_0_0_1px_var(--accent)]"
    @pointerenter="enter"
    @pointerleave="hovered = false"
    @focusin="focusIn"
    @focusout="focusOut"
  >
    <!-- Preview: a fixed 16:9 box, whatever fills it. -->
    <LinkWidget
      v-if="playing && content?.widget"
      :widget="content.widget"
      :title="title"
      :href="href"
    />
    <div
      v-else
      class="catalog-card-preview relative aspect-video w-full overflow-hidden bg-secondary"
    >
      <div
        v-if="loading"
        class="size-full animate-pulse bg-secondary motion-reduce:animate-none"
        role="status"
        :aria-label="`Loading a preview of ${card.host}`"
      />
      <button
        v-else-if="asset && !previewFailed"
        type="button"
        class="block size-full"
        :aria-label="`View ${asset.kind === 'video' ? 'video' : 'image'}: ${title}`"
        @click="viewing = true"
      >
        <video
          v-if="asset.kind === 'video'"
          :src="asset.url"
          class="size-full object-cover"
          preload="metadata"
          muted
          playsinline
          @error="previewFailed = true"
        />
        <span
          v-else-if="gif"
          class="relative block size-full [&>img]:size-full [&>img]:object-cover"
          ><StillImage
            :src="asset.url"
            :playing="animating"
            :width="asset.width"
            :height="asset.height"
            external
            @error="previewFailed = true"
        /></span>
        <NuxtImg
          v-else
          :src="asset.url"
          :width="asset.width || undefined"
          :height="asset.height || undefined"
          alt=""
          class="size-full object-cover"
          loading="lazy"
          decoding="async"
          referrerpolicy="no-referrer"
          @error="previewFailed = true"
        />
      </button>
      <button
        v-else-if="playable"
        type="button"
        class="grid size-full place-items-center"
        :aria-label="`Play: ${title}`"
        @click="playing = true"
      >
        <NuxtImg
          v-if="still && !previewFailed"
          :src="still"
          alt=""
          class="absolute inset-0 size-full object-cover"
          loading="lazy"
          decoding="async"
          referrerpolicy="no-referrer"
          @error="previewFailed = true"
        />
        <span
          class="relative grid size-12 place-items-center rounded-full bg-black/60 text-white shadow-control transition-transform group-hover:scale-105"
          ><AppIcon
            name="play"
            :size="22"
        /></span>
      </button>
      <a
        v-else-if="still && !previewFailed"
        :href="href"
        target="_blank"
        rel="noopener noreferrer nofollow"
        tabindex="-1"
        aria-hidden="true"
        class="relative block size-full [&>img]:size-full [&>img]:object-cover"
        ><StillImage
          v-if="stillGif"
          :src="still"
          :playing="animating"
          external
          @error="previewFailed = true" /><NuxtImg
          v-else
          :src="still"
          alt=""
          class="size-full object-cover"
          loading="lazy"
          decoding="async"
          referrerpolicy="no-referrer"
          @error="previewFailed = true"
      /></a>
      <div
        v-else
        class="grid size-full place-items-center bg-[radial-gradient(circle_at_30%_20%,color-mix(in_srgb,var(--accent)_14%,transparent),transparent_70%)] text-faint"
        aria-hidden="true"
      >
        <span class="grid justify-items-center gap-2"
          ><AppIcon
            :name="kindIcon(card.kind)"
            :size="30"
          /><span class="text-[length:var(--ui-caption)]">{{ card.host }}</span></span
        >
      </div>
      <span
        v-if="kindBadge[card.kind]"
        class="pointer-events-none absolute top-2 left-2 inline-flex items-center gap-1 rounded-sm bg-black/65 px-1.5 py-0.5 text-[length:var(--ui-label)] font-semibold text-white"
        ><AppIcon
          :name="kindIcon(card.kind)"
          :size="11"
        />{{ kindBadge[card.kind] }}</span
      >
      <div
        class="catalog-card-actions absolute top-2 right-2 flex gap-0.5 rounded-md bg-background/85 p-0.5 opacity-0 shadow-control backdrop-blur-sm transition-opacity group-hover:opacity-100 group-focus-within:opacity-100 group-[.is-selected]:opacity-100 [@media(hover:none)]:opacity-100"
      >
        <AppTooltip text="Copy link"
          ><UiButton
            variant="ghost"
            size="icon-sm"
            :aria-label="`Copy link: ${card.url}`"
            @click="copy(card.url)"
            ><AppIcon
              name="copy"
              :size="15" /></UiButton
        ></AppTooltip>
        <AppTooltip
          v-if="href"
          text="Open link"
          ><UiButton
            as="a"
            variant="ghost"
            size="icon-sm"
            :href="href"
            target="_blank"
            rel="noopener noreferrer nofollow"
            :aria-label="`Open link: ${card.url}`"
            ><AppIcon
              name="external"
              :size="15" /></UiButton
        ></AppTooltip>
      </div>
    </div>

    <div class="flex min-w-0 flex-1 flex-col gap-1 p-3">
      <span
        class="catalog-card-host flex min-w-0 items-center gap-1.5 text-[length:var(--ui-caption)] text-muted-foreground"
        ><NuxtImg
          v-if="icon && !iconFailed"
          :src="icon"
          width="14"
          height="14"
          alt=""
          class="size-3.5 shrink-0 rounded-[3px]"
          referrerpolicy="no-referrer"
          @error="iconFailed = true"
        /><AppIcon
          v-else
          name="link"
          :size="12"
          class="shrink-0"
        /><span class="truncate">{{ site }}</span></span
      >
      <a
        v-if="href"
        class="catalog-card-title line-clamp-2 text-[length:var(--ui-text)] font-semibold leading-snug text-foreground wrap-anywhere hover:underline"
        :href="href"
        target="_blank"
        rel="noopener noreferrer nofollow"
        >{{ title }}</a
      ><span
        v-else
        class="catalog-card-title line-clamp-2 text-[length:var(--ui-text)] font-semibold leading-snug text-foreground wrap-anywhere"
        >{{ title }}</span
      >
      <p
        v-if="description"
        class="catalog-card-description m-0 line-clamp-2 text-[length:var(--ui-small)] leading-snug text-muted-foreground wrap-anywhere"
      >
        {{ description }}
      </p>
      <p
        v-if="remark"
        class="catalog-card-excerpt m-0 mt-1 line-clamp-2 border-l-2 [border-left-style:solid] border-border pl-2 text-[length:var(--ui-small)] leading-snug text-body wrap-anywhere"
      >
        {{ remark }}
      </p>

      <div class="mt-auto flex min-w-0 items-center gap-2 pt-2">
        <button
          v-if="latest"
          type="button"
          class="catalog-card-source flex min-w-0 flex-1 items-center gap-2 rounded-sm text-left text-[length:var(--ui-caption)] text-muted-foreground hover:text-foreground"
          :aria-label="`Jump to the message: ${author(latest)} in ${place(latest)}`"
          @click="emit('jump', latest)"
        >
          <UserAvatar
            :member="chat.member(latest.senderId)"
            size="tiny"
          /><span class="min-w-0 truncate"
            ><strong class="font-semibold text-foreground">{{ author(latest) }}</strong> in
            {{ place(latest) }}</span
          >
        </button>
        <time
          class="shrink-0 text-[length:var(--ui-caption)] text-faint tabular-nums"
          :datetime="card.lastPostedAt"
          :title="`${full(card.lastPostedAt)} · ${latest ? networkName(latest.networkId) : ''}`"
          >{{ when(card.lastPostedAt) }}</time
        >
      </div>
      <!-- Repeated posts open as a menu, so a card never grows its grid row. -->
      <UiDropdownMenu v-if="card.count > 1">
        <UiDropdownMenuTrigger as-child>
          <UiButton
            variant="link"
            size="text"
            class="catalog-card-count -mx-2 self-start text-[length:var(--ui-caption)]"
            >Posted {{ card.count }} times<AppIcon
              name="down"
              :size="12"
          /></UiButton>
        </UiDropdownMenuTrigger>
        <UiDropdownMenuContent
          align="start"
          class="catalog-card-sources max-w-80 min-w-64"
          :aria-label="`Posts of ${linkTitle(card)}`"
        >
          <UiDropdownMenuItem
            v-for="source in card.sources"
            :key="source.messageId"
            class="items-start py-1.5"
            @select="emit('jump', source)"
          >
            <UserAvatar
              :member="chat.member(source.senderId)"
              size="tiny"
              class="mt-0.5"
            /><span class="flex min-w-0 flex-1 flex-col"
              ><span class="flex items-baseline gap-2"
                ><strong class="truncate font-semibold">{{ author(source) }}</strong
                ><span class="truncate opacity-75">{{ place(source) }}</span
                ><time
                  class="ml-auto shrink-0 text-[length:var(--ui-caption)] opacity-75 tabular-nums"
                  :datetime="source.postedAt"
                  :title="full(source.postedAt)"
                  >{{ when(source.postedAt) }}</time
                ></span
              ><span class="truncate text-[length:var(--ui-caption)] opacity-75">{{
                source.excerpt
              }}</span></span
            >
          </UiDropdownMenuItem>
          <UiDropdownMenuLabel
            v-if="card.count > card.sources.length"
            class="catalog-card-more font-normal"
          >
            and {{ card.count - card.sources.length }} earlier
            {{ card.count - card.sources.length === 1 ? 'post' : 'posts' }}
          </UiDropdownMenuLabel>
        </UiDropdownMenuContent>
      </UiDropdownMenu>
    </div>
    <MediaViewer
      v-if="viewing && asset"
      :items="assets"
      :label="title"
      :heading="title"
      :detail="latest ? `${author(latest)} in ${place(latest)}` : undefined"
      :source="href"
      @close="viewing = false"
    />
  </li>
</template>
