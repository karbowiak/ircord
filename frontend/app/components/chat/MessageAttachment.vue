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
const { context, renderer, style, containerStyle, blocked, showSensitiveMedia } = useEmbed({
  url: original,
  message: () => props.message,
  nearby: preload,
  visible,
  attachment: () => props.attachment,
  onReady: (url, ready) => emit('mediaReady', url, ready),
})
const { source, saved, refs } = useOfflineMedia(
  () => props.attachment.url,
  () => visible.value && !blocked.value,
)
const pinned = computed(() => refs.value.some((ref) => ref.id === props.message?.id && ref.pinned))
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
      <SensitiveMediaPlaceholder
        v-if="blocked"
        :state="blocked"
        @reveal="showSensitiveMedia"
      />
      <component
        :is="renderer"
        v-else
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
    <!-- Reserve the same footer as the revealed media without mounting its link or controls. -->
    <div
      v-if="!framed && blocked"
      aria-hidden="true"
      class="message-file invisible flex items-center gap-3 mt-1.5 px-3.5 py-2.5 w-full"
    >
      <span class="h-[28px] w-[28px] shrink-0" />
      <span class="flex-1 min-w-0 flex flex-col"><strong>&nbsp;</strong><small>&nbsp;</small></span>
      <span class="size-[18px] shrink-0" />
    </div>
    <a
      v-else-if="!framed"
      class="message-file flex items-center gap-3 mt-1.5 px-3.5 py-2.5 w-full rounded-lg [background:linear-gradient(180deg,color-mix(in_srgb,var(--raised)_65%,var(--card)),var(--card))] shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_85%,transparent)] transition-shadow hover:shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--accent)_45%,var(--border))] text-primary no-underline!"
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
    <div
      v-if="message && !framed && blocked"
      aria-hidden="true"
      class="size-7 [@media(pointer:coarse)]:size-11"
    />
    <AttachmentOfflineControls
      v-if="message && !framed && !blocked"
      :saved="saved"
      :pinned="pinned"
      :offline="offlineStatus.offline"
      :available="offlineAvailable"
      :saving="saving"
      @toggle="keep"
    />
  </div>
</template>
