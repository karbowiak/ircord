<script setup vapor lang="ts">
import type { Message } from '../../../shared/types/chat'
import { attachmentMediaAsset, messageAttachments } from '../../../shared/utils/link-media'
const props = defineProps<{ message: Message }>()
const emit = defineEmits<{ mediaReady: [url: string, ready: boolean] }>()
const entries = computed(() =>
  messageAttachments(props.message).map((attachment) => ({
    attachment,
    asset: attachmentMediaAsset(attachment, window.location.href),
  })),
)
const visual = computed(() =>
  entries.value
    .filter(({ asset }) => asset?.kind === 'image' || asset?.kind === 'video')
    .map(({ attachment }) => attachment),
)
const files = computed(() =>
  entries.value
    .filter(({ asset }) => asset?.kind !== 'image' && asset?.kind !== 'video')
    .map(({ attachment }) => attachment),
)
</script>
<template>
  <div class="message-attachments grid min-w-0 gap-2">
    <MessageAttachmentGallery
      v-if="visual.length > 1"
      :attachments="visual"
      :message="message"
    />
    <MessageAttachment
      v-else-if="visual[0]"
      :attachment="visual[0]"
      :message="message"
      @media-ready="(url, ready) => emit('mediaReady', url, ready)"
    />
    <MessageAttachment
      v-for="attachment in files"
      :key="attachment.url"
      :attachment="attachment"
      :message="message"
      @media-ready="(url, ready) => emit('mediaReady', url, ready)"
    />
  </div>
</template>
