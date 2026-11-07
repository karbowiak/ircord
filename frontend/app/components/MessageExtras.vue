<script setup lang="ts">
import type { Message } from '../../shared/types/chat'
import { mediaLinks } from '../../shared/utils/links'
import { stripFormatting } from '../../shared/utils/irc'
/** Attachments, reactions and delivery state under a message. */
const props = defineProps<{ message: Message }>()
const emit = defineEmits<{ image: [url: string, alt: string] }>()
const chat = useChatStore()
const preferences = usePreferencesStore()
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
          link.url !== card.value?.url,
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
const failed = reactive(new Set<string>())
</script>
<template>
  <div class="message-extras">
    <button
      v-if="message.attachment && isImage"
      class="message-image"
      :style="frame"
      :aria-label="`View ${message.attachment.name}`"
      @click="emit('image', message.attachment.url, message.attachment.alt)"
    >
      <img
        :src="message.attachment.url"
        :alt="message.attachment.alt"
        :width="message.attachment.width || undefined"
        :height="message.attachment.height || undefined"
        decoding="async"
      />
    </button>
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
    <a
      v-if="card"
      class="link-card"
      :href="card.url"
      target="_blank"
      rel="noopener noreferrer"
    >
      <span class="link-card-text"
        ><small>{{ card.siteName || card.host }}</small
        ><strong>{{ card.title }}</strong
        ><span v-if="card.description">{{ card.description }}</span></span
      >
      <img
        v-if="card.image && !failed.has(card.image)"
        :src="card.image"
        alt=""
        loading="lazy"
        decoding="async"
        referrerpolicy="no-referrer"
        @error="failed.add(card.image)"
      />
    </a>
    <template
      v-for="link in previews"
      :key="link.url"
    >
      <button
        v-if="link.kind === 'image' && !failed.has(link.url)"
        class="message-image link-preview"
        :aria-label="`View image from ${link.host}`"
        @click="emit('image', link.url, link.host)"
      >
        <img
          :src="link.url"
          alt=""
          loading="lazy"
          decoding="async"
          referrerpolicy="no-referrer"
          @error="failed.add(link.url)"
        />
      </button>
      <video
        v-else-if="link.kind === 'video' && !failed.has(link.url)"
        class="link-preview link-video"
        :src="link.url"
        controls
        preload="metadata"
        playsinline
        referrerpolicy="no-referrer"
        @error="failed.add(link.url)"
      />
    </template>
    <div
      v-if="message.reactions.length"
      class="reactions"
    >
      <button
        v-for="reaction in message.reactions"
        :key="reaction.emoji"
        class="reaction"
        :class="{ 'is-reacted': reaction.memberIds.includes('you') }"
        :aria-label="`${reaction.emoji}, ${reaction.memberIds.length} reactions`"
        :aria-pressed="reaction.memberIds.includes('you')"
        :title="reaction.memberIds.map((id) => chat.member(id)?.displayName).join(', ')"
        @click="chat.react(message, reaction.emoji)"
      >
        <MessageContent :text="reaction.emoji" /><span class="reaction-count">{{
          reaction.memberIds.length
        }}</span>
      </button>
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
