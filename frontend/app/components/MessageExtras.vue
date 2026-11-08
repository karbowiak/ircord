<script setup lang="ts">
import type { Message } from '../../shared/types/chat'
import { isAnimatedImageUrl, mediaLinks, type MediaLink } from '../../shared/utils/links'
import { stripFormatting } from '../../shared/utils/irc'
/** Attachments, reactions and delivery state under a message. */
const props = defineProps<{ message: Message }>()
const emit = defineEmits<{ image: [url: string, alt: string] }>()
const chat = useChatStore()
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
        stripFormatting(props.message.text),
        preferences.preferences.linkPreviews,
        location.origin,
      ).filter(
        (link) =>
          (!props.message.attachment || !link.url.endsWith(props.message.attachment.url)) &&
          link.url !== card.value?.url &&
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
    image: https(p.image) && preferences.preferences.linkPreviews !== 'off' ? p.image : undefined,
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
  if (failed.has(media.url)) return null
  return {
    url: media.url,
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
/** The card's image holds a still frame like any message image, playing while the card is hovered. */
const cardPlayback = useMediaPlayback()
const cardAnimated = computed(() => !!card.value?.image && isAnimatedImageUrl(card.value.image))
</script>
<template>
  <div class="message-extras">
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
    <a
      v-else-if="card"
      class="link-card"
      :href="card.url"
      target="_blank"
      rel="noopener noreferrer"
      v-on="cardPlayback.handlers"
    >
      <span class="link-card-text"
        ><small>{{ card.siteName || card.host }}</small
        ><strong>{{ card.title }}</strong
        ><span v-if="card.description">{{ card.description }}</span></span
      >
      <span
        v-if="card.image && !failed.has(card.image)"
        class="link-card-image media-frame"
        :class="{ 'is-playing': cardPlayback.playing.value }"
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
    <template
      v-for="link in previews"
      :key="link.url"
    >
      <MessageImage
        v-if="link.kind === 'image' && !failed.has(link.url)"
        class="link-preview"
        :src="link.url"
        :label="`View image from ${link.host}`"
        external
        @open="emit('image', link.url, link.host)"
        @error="failed.add(link.url)"
      />
      <MessageImage
        v-else-if="link.kind === 'gif'"
        class="link-preview"
        :src="link.url"
        :label="`View GIF from ${link.host}`"
        animated
        external
        :unavailable="`GIF unavailable · ${link.host}`"
        @open="emit('image', link.url, link.host)"
      />
      <MessageVideo
        v-else-if="link.kind === 'video'"
        :src="link.url"
        :host="link.host"
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
