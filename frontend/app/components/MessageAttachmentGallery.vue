<script setup lang="ts">
import type { Attachment, Message } from '../../shared/types/chat'
import { offlineArchive, offlineRevision, offlineStatus } from '../utils/offline-archive'
const props = defineProps<{ attachments: Attachment[]; message: Message }>()
const chat = useChatStore()
const { t } = useI18n()
const root = ref<HTMLElement | null>(null)
const nearby = ref(false)
const saved = ref(false)
const pinned = ref(false)
const saving = ref(false)
const available = computed(
  () =>
    !props.message.delivery &&
    offlineStatus.value.phase !== 'unavailable' &&
    offlineStatus.value.phase !== 'idle',
)
const { context, renderer, style } = useEmbed({
  url: () =>
    props.attachments[0] ? new URL(props.attachments[0].url, window.location.href).href : '',
  attachments: () => props.attachments,
  message: () => props.message,
  nearby,
})
let observer: IntersectionObserver | undefined
onMounted(() => {
  if (!('IntersectionObserver' in window)) {
    nearby.value = true
    return
  }
  observer = new IntersectionObserver(
    ([entry]) => {
      nearby.value = !!entry?.isIntersecting
    },
    {
      root: root.value?.closest('.timeline-scroll') ?? null,
      rootMargin: '150% 0px',
    },
  )
  if (root.value) observer.observe(root.value)
})
let revision = 0
watch(
  [
    () => props.attachments.map((attachment) => attachment.url).join('\n'),
    () => props.message.id,
    offlineRevision,
  ],
  async () => {
    const current = ++revision
    const messageId = props.message.id
    saved.value = pinned.value = false
    try {
      const files = await Promise.all(
        props.attachments.map((attachment) => offlineArchive.file(attachment.url)),
      )
      if (current !== revision) return
      saved.value = files.length > 0 && files.every((file) => !!file?.blob)
      pinned.value =
        files.length > 0 &&
        files.every((file) => file?.refs.some((ref) => ref.id === messageId && ref.pinned))
    } catch {
      /* The pin control stays available if storage becomes usable again. */
    }
  },
  { immediate: true },
)
async function keep() {
  saving.value = true
  try {
    await offlineArchive.pin(props.message, !pinned.value)
  } catch (error) {
    chat.notify(error instanceof Error ? error.message : t('mediaUi.attachment.saveFailed'))
  } finally {
    saving.value = false
  }
}
onBeforeUnmount(() => {
  revision++
  observer?.disconnect()
})
</script>
<template>
  <div
    ref="root"
    class="message-attachment-gallery group/media mt-2 min-w-0"
    :style="style"
  >
    <component
      :is="renderer"
      :embed="context"
      ><template #footer>
        <AttachmentOfflineControls
          :saved="saved"
          :pinned="pinned"
          :offline="offlineStatus.offline"
          :available="available"
          :saving="saving"
          @toggle="keep"
        /> </template
    ></component>
  </div>
</template>
