<script setup lang="ts">
import type { Message } from '../../shared/types/chat'
import { messageMediaLinks } from '../../shared/utils/link-media'
const props = defineProps<{ message: Message }>()
const emit = defineEmits<{ mediaReady: [url: string, ready: boolean] }>()
const container = ref<HTMLElement | null>(null)
const nearby = ref(false)
const links = computed(() =>
  props.message.deleted || props.message.attachment || props.message.attachments?.length
    ? []
    : messageMediaLinks(props.message.text),
)
let observer: IntersectionObserver | undefined
onMounted(() => {
  if (!('IntersectionObserver' in window)) {
    nearby.value = true
    return
  }
  // Every row reserves its URL-selected box, including optimistic messages. Resolve nearby
  // confirmed messages about a screen and a half ahead, inside the timeline's scroll region.
  observer = new IntersectionObserver(
    ([entry]) => {
      nearby.value = !!entry?.isIntersecting
    },
    { root: container.value?.closest('.timeline-scroll') ?? null, rootMargin: '150% 0px' },
  )
  watch(
    container,
    (element) => {
      observer?.disconnect()
      if (element) observer?.observe(element)
    },
    { immediate: true, flush: 'post' },
  )
})
onBeforeUnmount(() => observer?.disconnect())
</script>
<template>
  <div
    v-if="links.length"
    ref="container"
    class="message-links grid gap-3 min-w-0"
  >
    <MessageLink
      v-for="url in links"
      :key="`${message.id}:${url}`"
      :message="message"
      :url="url"
      :nearby="nearby"
      @media-ready="(link, ready) => emit('mediaReady', link, ready)"
    />
  </div>
</template>
