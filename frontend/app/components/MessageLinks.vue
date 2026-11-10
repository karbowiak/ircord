<script setup vapor lang="ts">
import type { Message } from '../../shared/types/chat'
import { messageMediaLinks } from '../../shared/utils/link-media'
const props = defineProps<{ message: Message }>()
const emit = defineEmits<{ mediaReady: [url: string, ready: boolean] }>()
const links = computed(() =>
  props.message.deleted || props.message.attachment || props.message.attachments?.length
    ? []
    : messageMediaLinks(props.message.text),
)
</script>
<template>
  <div
    v-if="links.length"
    class="message-links grid gap-3 min-w-0"
  >
    <MessageLink
      v-for="url in links"
      :key="`${message.clientId ?? message.id}:${url}`"
      :message="message"
      :url="url"
      @media-ready="(link, ready) => emit('mediaReady', link, ready)"
    />
  </div>
</template>
