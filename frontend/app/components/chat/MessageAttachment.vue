<script setup vapor lang="ts">
import { formatBytes } from '../../../shared/utils/bytes'
import type { Attachment, Message } from '../../../shared/types/chat'
import { offlineArchive, offlineStatus } from '../../utils/offline-archive'
import {
  attachmentMediaAsset,
  messageMediaLinks,
  publicMediaURL,
} from '../../../shared/utils/link-media'

const props = defineProps<{ attachment: Attachment; message?: Message }>()
const emit = defineEmits<{ mediaReady: [url: string, ready: boolean] }>()
const root = ref<HTMLElement | null>(null)
const { preload, visible } = useEmbedVisibility(root)
const { source, saved, refs } = useOfflineMedia(() => props.attachment.url, visible)
const pinned = computed(() => refs.value.some((ref) => ref.id === props.message?.id && ref.pinned))
const saving = ref(false)
const chat = useChatStore()
const { t, locale } = useI18n()
async function keep() {
  if (!props.message) return
  saving.value = true
  try {
    await offlineArchive.pin(props.message, !pinned.value)
  } catch (error) {
    chat.notify(error instanceof Error ? error.message : t('mediaUi.attachment.saveFailed'))
  } finally {
    saving.value = false
  }
}
const asset = computed(() => attachmentMediaAsset(props.attachment, window.location.href))
const framed = computed(() => asset.value?.kind === 'image')
const offlineAvailable = computed(
  () => offlineStatus.value.phase !== 'unavailable' && offlineStatus.value.phase !== 'idle',
)
const original = computed(
  () =>
    messageMediaLinks(props.message?.text ?? '').find(
      (url) => publicMediaURL(url) === asset.value?.url,
    ) ??
    asset.value?.url ??
    props.attachment.url,
)
const { context, renderer, style, containerStyle } = useEmbed({
  url: original,
  message: () => props.message,
  nearby: preload,
  visible,
  attachment: () => props.attachment,
  onReady: (url, ready) => emit('mediaReady', url, ready),
})
</script>
<template>
  <div
    ref="root"
    class="message-attachment grid gap-1.5 min-w-0"
    :style="containerStyle"
  >
    <div
      v-if="asset"
      class="message-link group/media mt-2 min-w-0 w-full"
      :style="style"
    >
      <component
        :is="renderer"
        :embed="context"
        ><template #footer>
          <AttachmentOfflineControls
            v-if="message"
            :saved="saved"
            :pinned="pinned"
            :offline="offlineStatus.offline"
            :available="offlineAvailable"
            :saving="saving"
            @toggle="keep"
          /> </template
      ></component>
    </div>
    <a
      v-if="!framed"
      class="message-file flex items-center gap-3 mt-1.5 px-3.5 py-2.5 w-full rounded-md bg-card shadow-[inset_0_0_0_1px_var(--border)] text-primary no-underline!"
      :href="source"
      target="_blank"
      rel="noopener noreferrer"
      :download="attachment.name"
    >
      <AppIcon
        name="file"
        :size="28"
      /><span class="flex-1 min-w-0 flex flex-col"
        ><strong class="text-primary truncate">{{ attachment.name }}</strong
        ><small class="text-faint">{{
          attachment.size ? formatBytes(attachment.size, locale) : t('mediaUi.attachment.fileType')
        }}</small></span
      ><AppIcon
        name="download"
        :size="18"
        class="text-muted-foreground"
      />
    </a>
    <AttachmentOfflineControls
      v-if="message && !framed"
      :saved="saved"
      :pinned="pinned"
      :offline="offlineStatus.offline"
      :available="offlineAvailable"
      :saving="saving"
      @toggle="keep"
    />
  </div>
</template>
