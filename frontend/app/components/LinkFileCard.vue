<script setup vapor lang="ts">
import type { components } from '../../shared/generated/api'
import { linkFileTypeKey, linkMediaReasonKey, publicMediaURL } from '../../shared/utils/link-media'

const props = defineProps<{
  file: components['schemas']['LinkMediaFile']
  url: string
  expired?: boolean
}>()
const { t, locale } = useI18n()
const original = computed(() => publicMediaURL(props.url))
const download = computed(() => !props.expired && publicMediaURL(props.file.url || ''))
const name = computed(() => props.file.name || t('mediaUi.attachment.fileType'))
const type = computed(() => {
  const key = linkFileTypeKey(props.file.contentType)
  return key ? t(key) : props.file.contentType || t('mediaUi.attachment.fileType')
})
const size = computed(() =>
  typeof props.file.size === 'number' && props.file.size > 0
    ? formatBytes(props.file.size, locale.value)
    : '',
)
const reason = computed(() =>
  props.expired
    ? t('mediaUi.linkPreview.expired')
    : props.file.reason
      ? t(linkMediaReasonKey(props.file.reason))
      : '',
)
</script>
<template>
  <!-- The same fixed box as a link card: name, type, a reserved status line and the action. -->
  <section
    class="link-file-card group/media relative grid min-w-0 grid-rows-[auto_18px_auto] content-start gap-2 overflow-hidden rounded-xl border border-solid border-border bg-secondary p-3 text-foreground"
    :style="{ height: 'var(--embed-height, 150px)' }"
  >
    <div class="flex min-w-0 items-start gap-3 pe-20">
      <AppIcon
        name="file"
        :size="28"
        class="mt-1 shrink-0 text-muted-foreground"
      />
      <div class="grid min-w-0 gap-1">
        <strong
          class="truncate text-[length:var(--ui-text)] font-semibold leading-5 wrap-anywhere"
          >{{ name }}</strong
        >
        <span class="truncate text-[length:var(--ui-caption)] leading-[18px] text-muted-foreground"
          >{{ type }}<template v-if="size"> · {{ size }}</template></span
        >
      </div>
    </div>
    <p
      class="m-0 truncate text-[length:var(--ui-caption)] leading-[18px] text-muted-foreground"
      :role="reason ? 'status' : undefined"
    >
      {{ reason }}
    </p>
    <UiButton
      v-if="download || original"
      as="a"
      variant="secondary"
      size="sm"
      class="justify-self-start"
      :href="download || original"
      :download="download ? name : undefined"
      target="_blank"
      rel="noopener noreferrer nofollow"
    >
      <AppIcon
        :name="download ? 'download' : 'external'"
        :size="15"
      />
      {{ t(download ? 'mediaUi.linkPreview.downloadFile' : 'mediaUi.linkPreview.openOriginal') }}
    </UiButton>
    <MediaLinkActions
      :url="url"
      always-visible
      class="absolute end-2 top-2"
    />
  </section>
</template>
