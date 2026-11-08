<script setup lang="ts">
import type { Attachment } from '../../shared/types/chat'
import { attachmentMediaAsset } from '../../shared/utils/link-media'

const props = defineProps<{ attachment: Attachment }>()
const root = ref<HTMLElement | null>(null)
const nearby = ref(false)
const failed = ref(false)
const asset = computed(() => attachmentMediaAsset(props.attachment, window.location.href))
let observer: IntersectionObserver | undefined
watch(
  () => props.attachment.url,
  () => {
    failed.value = false
  },
)
onMounted(() => {
  if (!('IntersectionObserver' in window)) {
    nearby.value = true
    return
  }
  // Load a screen and a half ahead within the timeline, as MessageLinks does.
  observer = new IntersectionObserver(
    ([entry]) => {
      nearby.value = !!entry?.isIntersecting
    },
    { root: root.value?.closest('.timeline-scroll') ?? null, rootMargin: '150% 0px' },
  )
  if (root.value) observer.observe(root.value)
})
onBeforeUnmount(() => observer?.disconnect())
</script>
<template>
  <div
    ref="root"
    class="message-attachment grid gap-1.5 min-w-0 w-[min(100%,_480px)]"
  >
    <div
      v-if="asset && !failed"
      class="message-link w-[min(100%,_480px)] min-w-0 mt-2"
    >
      <DirectLinkMedia
        :asset="asset"
        :description="attachment.alt || attachment.name"
        :nearby="nearby"
        @unavailable="failed = true"
      />
    </div>
    <p
      v-if="asset && failed"
      class="link-state flex flex-wrap items-center gap-2 text-muted-foreground text-[12px] min-h-7"
    >
      Preview unavailable. Download the file to view it.
    </p>
    <a
      class="message-file"
      :href="attachment.url"
      target="_blank"
      rel="noopener noreferrer"
      :download="attachment.name"
    >
      <AppIcon
        name="file"
        :size="28"
      /><span
        ><strong>{{ attachment.name }}</strong
        ><small>{{ attachment.size ? formatBytes(attachment.size) : 'File' }}</small></span
      ><AppIcon
        name="download"
        :size="18"
      />
    </a>
  </div>
</template>
