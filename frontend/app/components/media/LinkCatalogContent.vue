<!-- Not Vapor: Reka's `as-child` trigger merges its props into a VNode slot. -->
<script setup lang="ts">
import { useQueryCache } from '@pinia/colada'
import type { Attachment, LinkCard, LinkSource, MessagePage } from '../../../shared/types/chat'
import { messagesQuery } from '../../queries/chat'
import { linkTitle, openableLink } from '../../../shared/utils/link-catalog'
import {
  attachmentMediaAsset,
  linkMediaReasonKey,
  messageAttachments,
  publicMediaURL,
} from '../../../shared/utils/link-media'
import { offlineStatus } from '../../utils/offline-archive'
import { relativeTime } from '../../../shared/utils/devices'

/**
 * The visible content of a resource in Links: a preview resolved from its newest post (the same embed,
 * request and cache the timeline uses), what the page is, and who posted it where. The preview
 * is always a 16:9 box, so cards never change size as previews arrive.
 */
const props = defineProps<{ card: LinkCard }>()
const emit = defineEmits<{ jump: [source: LinkSource] }>()
const chat = useChatStore()
const { t, n, locale } = useI18n()
/** “3 days ago”: short enough to leave the author and channel room. The title has the date. */
const ago = (iso: string) =>
  relativeTime(iso, Date.now(), locale.value, (key, values) =>
    t(`mediaUi.devices.${key}`, values ?? {}),
  )
const $t = t
const { full, day, time } = useTimeFormat()
const when = (iso: string) => `${day(iso)} ${time(iso)}`

const conversation = (id: string) => chat.data?.conversations.find((c) => c.id === id)
const networkName = (id: string) =>
  chat.data?.networks.find((n) => n.id === id)?.name ?? t('mediaUi.linkCatalog.unknownNetwork')
const place = (s: LinkSource) => {
  const c = conversation(s.conversationId)
  return c ? `#${c.name}` : t('mediaUi.linkCatalog.closedChannel')
}
const author = (s: LinkSource) =>
  chat.member(s.senderId, s.networkId)?.displayName ||
  chat.member(s.senderId, s.networkId)?.nick ||
  t('mediaUi.linkCatalog.unknownAuthor')
const latest = computed(() => props.card.sources[0])
/** What the post said besides the link itself. */
const remark = computed(() => {
  const text = latest.value?.excerpt ?? ''
  return text
    .replace(/\bhttps?:\/\/\S+/gi, '')
    .replace(/[\s:–—-]+$/, '')
    .trim()
})

const queryCache = useQueryCache()
const catalogAttachment = computed<Attachment | undefined>(() => {
  const src = latest.value
  if (src) {
    const page = queryCache.getQueryData<MessagePage>(messagesQuery(src.conversationId).key)
    const msg = page?.items.find((item) => item.id === src.messageId)
    if (msg) {
      const target = publicMediaURL(props.card.url)
      const found = messageAttachments(msg).find(
        (item) => publicMediaURL(item.url) === target || item.url === props.card.url,
      )
      if (found) return found
    }
  }
  try {
    const parsed = new URL(props.card.url)
    const match = parsed.pathname.match(/\/files\/[0-9a-f]{32}\/[0-9a-f]{48}\/([^/?#]+)$/i)
    if (match?.[1]) {
      const name = decodeURIComponent(match[1])
      const asset = attachmentMediaAsset(
        { alt: '', name, url: props.card.url, width: 0, height: 0 },
        window.location.href,
      )
      if (asset) {
        return {
          alt: '',
          name,
          url: asset.url,
          contentType: asset.contentType,
          width: 0,
          height: 0,
        }
      }
    }
  } catch {
    // Ignore malformed URLs.
  }
  return undefined
})

// Metadata can arrive ahead of scrolling; actual players use the smaller render zone.
const root = ref<HTMLElement | null>(null)
const { preload, visible } = useEmbedVisibility(root)
/** The timeline's embed for the newest post, in the catalog's fixed 16:9 box. */
const {
  context: embed,
  renderer,
  style: embedStyle,
  content,
  expired,
  pending,
} = useEmbed({
  url: () => props.card.url,
  message: () =>
    latest.value
      ? { id: latest.value.messageId, conversationId: latest.value.conversationId }
      : undefined,
  attachment: catalogAttachment,
  nearby: preload,
  visible,
  surface: 'catalog',
})
/**
 * A still picture carries the site and title on it, Hibiki style. Videos keep their controls
 * clear, and generated cards and players already show their own text.
 */
const overlaid = computed(
  () =>
    embed.value.assets.length > 0 &&
    embed.value.assets.every((asset) => asset.kind === 'image') &&
    embed.value.content?.kind !== 'widget' &&
    !!title.value,
)
/** A link card without a picture draws its site and title as a graphic. */
const titledGraphic = computed(
  () =>
    !embed.value.assets.length &&
    !!embed.value.card &&
    !embed.value.card.image &&
    !embed.value.widget &&
    !embed.value.file,
)
/** The preview already shows the site and title, so the body leaves them out. */
const titledPreview = computed(() => overlaid.value || titledGraphic.value)
/** The page description, unless the graphic shows it or a more useful line takes its place. */
const shownDescription = computed(
  () => !!description.value && !remark.value && !reason.value && !titledGraphic.value,
)
const icon = computed(
  () =>
    !expired.value &&
    !offlineStatus.value.offline &&
    content.value?.card?.icon &&
    publicMediaURL(content.value.card.icon),
)
const title = computed(
  () =>
    content.value?.card?.title ||
    content.value?.title ||
    catalogAttachment.value?.name ||
    linkTitle(props.card),
)
const description = computed(() => content.value?.card?.description || content.value?.description)
const site = computed(() => content.value?.card?.site || props.card.host)
const loading = computed(() => preload.value && pending.value && !content.value)
const reason = computed(() => {
  if (catalogAttachment.value) return ''
  if (offlineStatus.value.offline) return t('workspaceUi.links.offline')
  if (loading.value) return t('mediaUi.linkPreview.loading')
  if (expired.value) return t('mediaUi.linkPreview.expired')
  if (embed.value.failed) return t(linkMediaReasonKey('fetch_failed'))
  const code = content.value?.card?.reason
  if (!code || code === 'no_oembed') return ''
  return t(linkMediaReasonKey(code))
})
const iconFailed = ref(false)
watch(
  () => props.card.url,
  () => {
    iconFailed.value = false
  },
)

const href = computed(() => (openableLink(props.card.url) ? props.card.url : undefined))
</script>
<template>
  <li
    ref="root"
    class="catalog-card group group/media relative flex min-w-0 flex-col overflow-hidden rounded-[14px] border border-solid border-border/60 bg-card [background:linear-gradient(180deg,color-mix(in_srgb,var(--raised)_45%,var(--card)),var(--card))] transition-[border-color,box-shadow] hover:border-primary/45 focus-within:border-border [&.is-selected]:border-primary [&.is-selected]:shadow-[0_0_0_1px_var(--accent)]"
  >
    <!-- Preview: always a 16:9 box (the embed style), whatever the renderer puts in it. -->
    <div
      class="catalog-card-preview relative overflow-hidden bg-secondary [&_.direct-media]:rounded-none [&_.direct-media-frame>img]:object-center! [&_.link-card]:rounded-none [&_.link-card]:border-0 [&_.link-file-card]:rounded-none [&_.link-file-card]:border-0 [&_.link-widget]:rounded-none [&_.media-gallery-mosaic]:rounded-none [&_.media-still]:object-center!"
      :style="embedStyle"
    >
      <component
        :is="renderer"
        :embed="embed"
      />
      <div
        v-if="!embed.assets.some((a) => a.kind === 'video' || a.kind === 'audio')"
        aria-hidden="true"
        class="pointer-events-none absolute inset-x-0 bottom-0 flex items-end gap-3 p-2.5"
        :class="
          overlaid
            ? 'h-[70%] bg-gradient-to-t from-black/85 via-black/45 to-transparent pl-3.5'
            : 'h-16 justify-end bg-gradient-to-t from-black/55 via-black/20 to-transparent'
        "
      >
        <span
          v-if="overlaid"
          class="flex min-w-0 flex-1 flex-col gap-0.5 text-white [text-shadow:0_1px_8px_rgb(0_0_0/45%)]"
          ><span class="truncate text-[length:var(--ui-caption)] text-white/75">{{ site }}</span
          ><span
            class="line-clamp-2 text-[length:var(--ui-text)] leading-snug font-[700] wrap-anywhere"
            >{{ title }}</span
          ></span
        >
        <span
          class="grid size-8 place-items-center rounded-full bg-primary text-primary-foreground shadow-[0_6px_16px_color-mix(in_srgb,var(--accent)_42%,transparent)] transition-transform duration-150 group-hover:scale-105"
        >
          <AppIcon
            :name="embed.content?.kind === 'widget' ? 'play' : 'external'"
            :size="14"
          />
        </span>
      </div>
    </div>

    <div class="catalog-card-body flex min-w-0 flex-col gap-1 px-3 py-2.5 [&>*]:shrink-0">
      <!-- On a pictured card the site and title are on the picture; a repost count stays. -->
      <div
        v-if="!titledPreview || card.count > 1"
        class="catalog-card-host flex min-w-0 items-center gap-1.5 text-[length:var(--ui-caption)] text-muted-foreground"
        :class="{ 'justify-end': titledPreview }"
      >
        <template v-if="!titledPreview"
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
          /><span class="min-w-0 truncate">{{ site }}</span></template
        >
        <!-- Repeated posts open as a menu, so a card never grows its grid row. -->
        <UiDropdownMenu v-if="card.count > 1">
          <UiDropdownMenuTrigger as-child>
            <UiButton
              variant="link"
              size="text"
              class="catalog-card-count ml-auto h-auto shrink-0 p-0 text-[length:var(--ui-caption)]"
              >{{ t('mediaUi.linkCatalog.postedTimes', { count: n(card.count) }, card.count)
              }}<AppIcon
                name="down"
                :size="12"
            /></UiButton>
          </UiDropdownMenuTrigger>
          <UiDropdownMenuContent
            align="end"
            class="catalog-card-sources max-w-80 min-w-64"
            :aria-label="$t('mediaUi.linkCatalog.postsOf', { title: linkTitle(card) })"
          >
            <UiDropdownMenuItem
              v-for="source in card.sources"
              :key="source.messageId"
              class="items-start py-1.5"
              @select="emit('jump', source)"
            >
              <UserAvatar
                :member="chat.member(source.senderId, source.networkId)"
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
              {{
                t(
                  'mediaUi.linkCatalog.earlierPosts',
                  { count: n(card.count - card.sources.length) },
                  card.count - card.sources.length,
                )
              }}
            </UiDropdownMenuLabel>
          </UiDropdownMenuContent>
        </UiDropdownMenu>
      </div>
      <!-- Read out on pictured cards, where the visible title is on the picture. -->
      <a
        v-if="href"
        class="catalog-card-title line-clamp-2 text-[length:var(--ui-text)] font-semibold leading-snug text-foreground wrap-anywhere hover:underline"
        :class="{ 'sr-only': titledPreview }"
        :href="href"
        target="_blank"
        rel="noopener noreferrer nofollow"
        >{{ title }}</a
      ><span
        v-else
        class="catalog-card-title line-clamp-2 text-[length:var(--ui-text)] font-semibold leading-snug text-foreground wrap-anywhere"
        :class="{ 'sr-only': titledPreview }"
        >{{ title }}</span
      >
      <!-- One line beside the title at most: the shared message, then the preview's status,
           then the page description. -->
      <p
        v-if="shownDescription"
        class="catalog-card-description m-0 line-clamp-1 text-[length:var(--ui-small)] leading-snug text-muted-foreground wrap-anywhere"
      >
        {{ description }}
      </p>
      <p
        v-if="reason && !remark"
        class="m-0 line-clamp-1 text-[length:var(--ui-caption)] text-muted-foreground wrap-anywhere"
      >
        {{ reason }}
      </p>
      <p
        v-if="remark"
        class="catalog-card-excerpt m-0 mt-0.5 line-clamp-1 border-l-2 [border-left-style:solid] border-border pl-2 text-[length:var(--ui-small)] leading-snug text-body wrap-anywhere"
      >
        {{ remark }}
      </p>

      <div
        class="mt-auto flex min-w-0 items-center gap-2"
        :class="{
          'border-t border-border/50 pt-1.5':
            !titledPreview || remark || reason || shownDescription,
        }"
      >
        <button
          v-if="latest"
          type="button"
          class="catalog-card-source -mx-1.5 flex min-w-0 flex-1 items-center gap-2 rounded-full px-1.5 py-0.5 text-left text-[length:var(--ui-caption)] text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
          :aria-label="
            $t('mediaUi.linkCatalog.jumpToMessage', {
              author: author(latest),
              place: place(latest),
            })
          "
          @click="emit('jump', latest)"
        >
          <UserAvatar
            :member="chat.member(latest.senderId, latest.networkId)"
            size="tiny"
          /><i18n-t
            keypath="mediaUi.linkCatalog.sourceCaption"
            tag="span"
            class="min-w-0 truncate"
            ><template #author
              ><strong class="font-semibold text-foreground">{{ author(latest) }}</strong></template
            ><template #place>{{ place(latest) }}</template></i18n-t
          >
        </button>
        <time
          class="shrink-0 text-[length:var(--ui-caption)] text-faint tabular-nums"
          :datetime="card.lastPostedAt"
          :title="
            latest
              ? t('mediaUi.linkCatalog.timeWithNetwork', {
                  time: full(card.lastPostedAt),
                  network: networkName(latest.networkId),
                })
              : t('mediaUi.linkCatalog.timeOnly', { time: full(card.lastPostedAt) })
          "
          >{{ ago(card.lastPostedAt) }}</time
        >
      </div>
    </div>
  </li>
</template>
