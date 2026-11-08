<script setup lang="ts">
import type { Message, ViewerItem } from '../../shared/types/chat'
import {
  groupMedia,
  isAnimatedImageUrl,
  maxMediaLinks,
  maxPlayers,
  mediaLinks,
  youtubeVideo,
  youtubeThumbnail,
  type MediaLink,
} from '../../shared/utils/links'
import { textOutsideCode } from '../../shared/utils/irc'
/**
 * Attachments, link embeds, reactions and delivery state under a message. Links become typed
 * embeds: YouTube players, audio and video players, single images or a gallery of several, and
 * the server's page card (a website, an article with its lead image, or an album's images).
 */
const props = defineProps<{ message: Message }>()
const emit = defineEmits<{ image: [url: string, alt: string, set?: ViewerItem[]] }>()
const chat = useChatStore()
const hosted = useHostedMedia(() => props.message)
const reactionSupported = computed(() => chat.messageActionSupported(props.message, 'reactions'))
const reactionReason = computed(() => chat.messageActionReason(props.message, 'reactions'))
const preferences = usePreferencesStore()
/** Uploads declared animated by type or name; other image uploads are checked by their bytes. */
const isGif = computed(() => {
  const a = props.message.attachment
  return (
    !!a &&
    (a.contentType === 'image/gif' ||
      a.contentType === 'image/apng' ||
      (!a.contentType && /\.(gif|apng)$/i.test(a.name)))
  )
})
const isImage = computed(
  () =>
    !props.message.attachment?.contentType ||
    props.message.attachment.contentType.startsWith('image/'),
)
/** Known dimensions reserve the exact space; unknown ones get a fixed frame, so rows never jump. */
const frame = computed(() => {
  const a = props.message.attachment
  return a?.width && a.height ? { aspectRatio: `${a.width} / ${a.height}` } : { height: '240px' }
})
const previews = computed(() =>
  props.message.gif
    ? []
    : mediaLinks(
        textOutsideCode(props.message.text),
        preferences.preferences.linkPreviews,
        location.origin,
        maxMediaLinks,
        hosted.enabled.value,
      )
        .flatMap((link) => {
          if (!hosted.enabled.value || !['image', 'gif', 'video'].includes(link.kind)) return [link]
          const url = hosted.source(link.url)
          if (!url) return []
          const record = hosted.records.value.get(link.url)
          return [
            {
              ...link,
              url,
              kind:
                record?.kind === 'video'
                  ? ('video' as const)
                  : record?.animated
                    ? ('gif' as const)
                    : ('image' as const),
            },
          ]
        })
        .filter(
          (link) =>
            (!props.message.attachment || !link.url.endsWith(props.message.attachment.url)) &&
            // A YouTube link merges with its card below instead of being dropped for it.
            (link.kind === 'youtube' || link.url !== card.value?.url) &&
            // The resolved GIF, linked directly or through its share link (tenor.com/xyz.gif),
            // would otherwise preview twice.
            link.url !== sharedMedia.value?.url &&
            !(sharedMedia.value && link.host === new URL(card.value!.url).hostname),
        ),
)
const https = (value?: string) => !!value && /^https:\/\/[^\s]+$/i.test(value)
/**
 * A page preview fetched by the backend (when the conversation opts in). Its text costs nothing;
 * the image loads from the linked site, so it follows the device's preview setting.
 */
const card = computed(() => {
  const p = props.message.preview
  if (props.message.previewSource !== 'server' || !p?.title || !https(p.url)) return null
  return {
    ...p,
    host: new URL(p.url).hostname.replace(/^www\./, ''),
    image:
      https(p.image) && preferences.preferences.linkPreviews !== 'off'
        ? hosted.source(p.image)
        : undefined,
    images: p.images?.flatMap((image) => {
      const url = hosted.source(image.url)
      return url ? [{ ...image, url, animated: hosted.records.value.get(image.url)?.animated }] : []
    }),
  }
})
/** Media that failed to load; they fall back to the card or disappear. */
const failed = reactive(new Set<string>())
/**
 * Media the backend resolved from a supported share page (a Tenor or GIPHY link). It loads from
 * the provider's media host, so like card images it follows the device's preview setting;
 * when previews are off or it fails to load, the text card is shown instead.
 */
const sharedMedia = computed<MediaLink | null>(() => {
  const media = card.value && props.message.preview?.media
  if (!media || !https(media.url) || preferences.preferences.linkPreviews === 'off') return null
  const url = hosted.source(media.url)
  if (!url || failed.has(url)) return null
  return {
    url,
    host: card.value!.siteName || card.value!.host,
    kind: media.kind,
    width: media.width,
    height: media.height,
  }
})
/** Declared sizes reserve the exact frame (at most 300px tall); otherwise the fixed frame applies. */
const linkFrame = (link: MediaLink) =>
  link.width && link.height
    ? {
        aspectRatio: `${link.width} / ${link.height}`,
        height: 'auto',
        width: `min(400px, 100%, ${Math.round((300 * link.width) / link.height)}px)`,
      }
    : undefined
const previewsOn = computed(() => preferences.preferences.linkPreviews !== 'off')
/**
 * The YouTube video the server card describes. With previews on it becomes the player (taking the
 * card's title, channel and embedding permission) instead of a second card; with previews off
 * only the text card remains. Cards stored before typed embeds have no `embed`, only a YouTube
 * URL; they merge into the player the same way.
 */
const cardVideo = computed(() => {
  if (!previewsOn.value || !card.value) return null
  const embed = card.value.embed
  if (embed?.provider !== 'youtube') return youtubeVideo(card.value.url)
  const video = youtubeVideo(`https://youtu.be/${embed.id}`)
  return video ? { ...video, start: embed.start ?? 0, short: !!embed.short } : null
})
const media = computed(() => groupMedia(previews.value))
/** Players in link order; the card's video joins them when its link was not recognised itself. */
const players = computed(() => {
  const list = media.value.players
  const video = cardVideo.value
  if (!video || list.some((link) => link.youtube?.id === video.id)) return list
  const link: MediaLink = { url: card.value!.url, host: 'YouTube', kind: 'youtube', youtube: video }
  return [link, ...list].slice(0, maxPlayers)
})
const youtubeDetails = (link: MediaLink) =>
  cardVideo.value && link.youtube?.id === cardVideo.value.id
    ? {
        title: card.value!.title,
        author: card.value!.author,
        thumbnail: hosted.enabled.value
          ? (hosted.source(props.message.preview?.image || youtubeThumbnail(link.youtube!.id)) ??
            null)
          : undefined,
        ...(card.value!.embed?.provider === 'youtube'
          ? {
              embeddable: card.value!.embed.embeddable,
              unavailable: !!card.value!.embed.unavailable,
            }
          : {}),
      }
    : hosted.enabled.value && link.youtube
      ? { thumbnail: hosted.source(youtubeThumbnail(link.youtube.id)) ?? null }
      : {}
const viewerSet = (links: MediaLink[]): ViewerItem[] =>
  links.map((link) => ({ url: link.url, name: link.host }))
/** An album page's images, previewed like card images. */
const cardGallery = computed(() => {
  const images = card.value?.images?.filter((image) => https(image.url))
  if (!previewsOn.value || !images || images.length < 2) return null
  return images.map((image) => ({
    url: image.url,
    name: card.value!.title,
    animated: image.animated,
  }))
})
const article = computed(() => card.value?.kind === 'article')
/** Article lead images reserve their declared shape, bounded like other message media. */
const leadFrame = computed(() => {
  const c = card.value
  return c?.imageWidth && c.imageHeight
    ? { aspectRatio: `${c.imageWidth} / ${c.imageHeight}` }
    : undefined
})
/** The card's image holds a still frame like any message image, playing while the card is hovered. */
const cardPlayback = useMediaPlayback()
const cardAnimated = computed(
  () =>
    !!props.message.preview?.image &&
    (hosted.records.value.get(props.message.preview.image)?.animated ||
      isAnimatedImageUrl(props.message.preview.image)),
)
</script>
<template>
  <div class="message-extras">
    <HostedMediaStatus
      v-if="hosted.enabled.value && !message.gif && preferences.preferences.linkPreviews !== 'off'"
      :items="message.media ?? []"
    />
    <MessageImage
      v-if="message.attachment && isImage"
      :style="frame"
      :src="message.attachment.url"
      :alt="message.attachment.alt"
      :label="`View ${message.attachment.name}`"
      :width="message.attachment.width"
      :height="message.attachment.height"
      :animated="isGif"
      :unavailable="`Image unavailable · ${message.attachment.name}`"
      @open="emit('image', message.attachment.url, message.attachment.alt)"
    />
    <a
      v-else-if="message.attachment"
      class="message-file"
      :href="message.attachment.url"
      target="_blank"
      rel="noopener noreferrer"
      :download="message.attachment.name"
    >
      <AppIcon
        name="file"
        :size="28"
      /><span
        ><strong>{{ message.attachment.name }}</strong
        ><small>{{
          message.attachment.size ? formatBytes(message.attachment.size) : 'File'
        }}</small></span
      ><AppIcon
        name="download"
        :size="18"
      />
    </a>
    <MessageImage
      v-if="sharedMedia"
      class="link-preview"
      :style="linkFrame(sharedMedia)"
      :src="sharedMedia.url"
      :label="`View GIF from ${sharedMedia.host}`"
      :width="sharedMedia.width"
      :height="sharedMedia.height"
      animated
      external
      @open="emit('image', sharedMedia.url, sharedMedia.host)"
      @error="failed.add(sharedMedia.url)"
    />
    <template v-else-if="card && !cardVideo">
      <a
        class="link-card"
        :class="{ 'is-article': article }"
        :href="card.url"
        target="_blank"
        rel="noopener noreferrer"
        v-on="cardPlayback.handlers"
      >
        <span class="link-card-text"
          ><small
            >{{ card.siteName || card.host
            }}<template v-if="card.author"> · {{ card.author }}</template></small
          ><strong>{{ card.title }}</strong
          ><span v-if="card.description">{{ card.description }}</span></span
        >
        <span
          v-if="card.image && !cardGallery && !failed.has(card.image)"
          class="link-card-image media-frame"
          :class="{ 'is-playing': cardPlayback.playing.value }"
          :style="article ? leadFrame : undefined"
        >
          <StillImage
            :src="card.image"
            :playing="cardPlayback.playing.value"
            external
            @error="failed.add(card.image)"
          />
          <span
            v-if="cardAnimated"
            class="media-kind"
            aria-hidden="true"
            >GIF</span
          >
        </span>
      </a>
      <MessageGallery
        v-if="cardGallery"
        :items="cardGallery"
        :label="`${cardGallery.length} images from ${card.siteName || card.host}`"
        @open="(index) => emit('image', cardGallery![index]!.url, card!.title, cardGallery!)"
      />
    </template>
    <template
      v-for="link in players"
      :key="link.url"
    >
      <YouTubeEmbed
        v-if="link.kind === 'youtube' && link.youtube"
        :video="link.youtube"
        v-bind="youtubeDetails(link)"
      />
      <MessageVideo
        v-else-if="link.kind === 'video'"
        :src="link.url"
        :host="link.host"
      />
      <MessageAudio
        v-else-if="link.kind === 'audio'"
        :src="link.url"
        :host="link.host"
      />
    </template>
    <MessageGallery
      v-if="media.images.length > 1"
      :items="
        media.images.map((link) => ({ ...link, name: link.host, animated: link.kind === 'gif' }))
      "
      :label="`${media.images.length} images`"
      @open="
        (index) =>
          emit(
            'image',
            media.images[index]!.url,
            media.images[index]!.host,
            viewerSet(media.images),
          )
      "
    />
    <template v-else-if="media.images[0]">
      <MessageImage
        v-if="media.images[0].kind === 'image' && !failed.has(media.images[0].url)"
        class="link-preview"
        :src="media.images[0].url"
        :label="`View image from ${media.images[0].host}`"
        external
        @open="emit('image', media.images[0].url, media.images[0].host)"
        @error="failed.add(media.images[0].url)"
      />
      <MessageImage
        v-else-if="media.images[0].kind === 'gif'"
        class="link-preview"
        :src="media.images[0].url"
        :label="`View GIF from ${media.images[0].host}`"
        animated
        external
        :unavailable="`GIF unavailable · ${media.images[0].host}`"
        @open="emit('image', media.images[0].url, media.images[0].host)"
      />
    </template>
    <div
      v-if="message.reactions.length"
      class="reactions"
    >
      <component
        v-for="reaction in message.reactions"
        :is="reactionSupported ? 'button' : 'span'"
        :key="reaction.emoji"
        class="reaction"
        :class="{ 'is-reacted': reaction.memberIds.includes('you') }"
        :aria-label="`${reaction.emoji}, ${reaction.memberIds.length} reactions`"
        :aria-pressed="reactionSupported ? reaction.memberIds.includes('you') : undefined"
        :disabled="reactionSupported ? !!reactionReason : undefined"
        :title="
          reactionReason ?? reaction.memberIds.map((id) => chat.member(id)?.displayName).join(', ')
        "
        @click="reactionSupported && chat.react(message, reaction.emoji)"
      >
        <MessageContent :text="reaction.emoji" /><span class="reaction-count">{{
          reaction.memberIds.length
        }}</span>
      </component>
    </div>
    <div
      v-if="message.delivery === 'failed'"
      class="delivery-error"
    >
      Not delivered. <button @click="chat.retry(message)">Retry</button> ·
      <button @click="chat.discard(message)">Discard</button>
    </div>
  </div>
</template>
