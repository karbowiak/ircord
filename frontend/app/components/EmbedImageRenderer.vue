<script setup vapor lang="ts">
import type { CSSProperties } from 'vue'
import type { EmbedContext } from '../types/embed'
const props = defineProps<{ embed: EmbedContext }>()
const { t } = useI18n()
/** The row stays reserved; only this inner rectangle follows the validated media dimensions. */
const mediaBox = computed<CSSProperties>(() => {
  const asset = props.embed.assets[0]
  const width = asset?.width ?? props.embed.attachment?.width
  const height = asset?.height ?? props.embed.attachment?.height
  if (
    !width ||
    !height ||
    !Number.isFinite(width) ||
    !Number.isFinite(height) ||
    width <= 0 ||
    height <= 0
  )
    return { width: '100%', maxWidth: '480px', height: '100%' }
  const ratio = width / height
  return {
    width: `min(100cqw, calc(100cqh * ${ratio}))`,
    height: `min(100cqh, calc(100cqw / ${ratio}))`,
  }
})
const filename = computed(() => {
  if (props.embed.attachment) return props.embed.attachment.name
  try {
    return (
      decodeURIComponent(new URL(props.embed.url).pathname.split('/').at(-1) || '') ||
      props.embed.host
    )
  } catch {
    return props.embed.host
  }
})
</script>
<template>
  <div class="absolute inset-0 min-h-0 min-w-0 [container-type:size]">
    <div
      class="relative min-h-0 min-w-0"
      :style="mediaBox"
    >
      <EmbedMediaRenderer
        :embed="embed"
        :actions="false"
        plain
      />
      <div
        class="absolute end-2 top-2 z-3 flex items-center gap-0.5 rounded-md bg-background/95 p-0.5 text-foreground shadow-control"
      >
        <UiButton
          as="a"
          :href="embed.downloadSource"
          :download="filename"
          target="_blank"
          rel="noopener noreferrer"
          variant="ghost"
          size="icon-sm"
          :disabled="!embed.downloadSource"
          :aria-label="t('mediaUi.linkPreview.downloadFile')"
        >
          <AppIcon
            name="download"
            :size="15"
          />
        </UiButton>
        <MediaLinkActions
          :url="embed.url"
          always-visible
          class="bg-transparent! p-0! shadow-none!"
        />
      </div>
      <div
        v-if="embed.attachment && embed.surface !== 'catalog'"
        class="absolute bottom-2 start-2 z-3"
      >
        <slot name="footer" />
      </div>
    </div>
  </div>
</template>
