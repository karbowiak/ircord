<script setup vapor lang="ts">
import type { Message } from '../../../shared/types/chat'

const props = defineProps<{ message: Message; url: string }>()
const emit = defineEmits<{ mediaReady: [url: string, ready: boolean] }>()
const root = ref<HTMLElement | null>(null)
const { preload, visible } = useEmbedVisibility(root)
const { context, renderer, style } = useEmbed({
  message: () => props.message,
  url: () => props.url,
  nearby: preload,
  visible,
  onReady: (url, ready) => emit('mediaReady', url, ready),
})
</script>
<template>
  <div
    ref="root"
    class="message-link group/media mt-2 min-w-0"
    :style="style"
    :aria-busy="context.pending || undefined"
  >
    <component
      :is="renderer"
      :embed="context"
    />
  </div>
</template>
