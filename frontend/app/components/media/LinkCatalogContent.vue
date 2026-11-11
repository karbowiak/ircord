<script setup lang="ts" vapor>
import type { LinkCard, LinkSource } from '../../../shared/types/chat'
import { linkTitle, openableLink } from '../../../shared/utils/link-catalog'
import { linkMediaReasonKey, publicMediaURL } from '../../../shared/utils/link-media'
import { offlineStatus } from '../../utils/offline-archive'

/**
 * The visible content of a resource in Links: a preview resolved from its newest post (the same embed,
 * request and cache the timeline uses), what the page is, and who posted it where. The preview
 * is always a 16:9 box, so cards never change size as previews arrive.
 */
const props = defineProps<{ card: LinkCard }>()
const emit = defineEmits<{ jump: [source: LinkSource] }>()
const chat = useChatStore()
const { t, n } = useI18n()
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
  nearby: preload,
  visible,
  surface: 'catalog',
})
const icon = computed(
  () =>
    !expired.value &&
    !offlineStatus.value.offline &&
    content.value?.card?.icon &&
    publicMediaURL(content.value.card.icon),
)
const title = computed(
  () => content.value?.card?.title || content.value?.title || linkTitle(props.card),
)
const description = computed(() => content.value?.card?.description || content.value?.description)
const site = computed(() => content.value?.card?.site || props.card.host)
const loading = computed(() => preload.value && pending.value && !content.value)
const reason = computed(() => {
  if (offlineStatus.value.offline) return t('workspaceUi.links.offline')
  if (loading.value) return t('mediaUi.linkPreview.loading')
  if (expired.value) return t('mediaUi.linkPreview.expired')
  if (embed.value.failed) return t(linkMediaReasonKey('fetch_failed'))
  return content.value?.card?.reason ? t(linkMediaReasonKey(content.value.card.reason)) : ''
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
    class="catalog-card group group/media relative flex min-w-0 flex-col overflow-hidden rounded-lg border border-solid border-border/60 bg-card transition-[border-color,box-shadow] hover:border-border focus-within:border-border [&.is-selected]:border-primary [&.is-selected]:shadow-[0_0_0_1px_var(--accent)]"
  >
    <!-- Preview: always a 16:9 box (the embed style), whatever the renderer puts in it. -->
    <div
      class="catalog-card-preview bg-secondary [&_.direct-media]:rounded-none [&_.link-card]:rounded-none [&_.link-card]:border-0 [&_.link-file-card]:rounded-none [&_.link-file-card]:border-0 [&_.link-widget]:rounded-none [&_.media-gallery-mosaic]:rounded-none"
      :style="embedStyle"
    >
      <component
        :is="renderer"
        :embed="embed"
      />
    </div>

    <div class="catalog-card-body flex min-w-0 flex-col gap-1 p-3">
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
        v-if="reason"
        class="m-0 line-clamp-2 text-[length:var(--ui-caption)] text-muted-foreground wrap-anywhere"
      >
        {{ reason }}
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
            >{{ t('mediaUi.linkCatalog.postedTimes', { count: n(card.count) }, card.count)
            }}<AppIcon
              name="down"
              :size="12"
          /></UiButton>
        </UiDropdownMenuTrigger>
        <UiDropdownMenuContent
          align="start"
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
  </li>
</template>
