<script setup lang="ts">
/**
 * The downsides of read-aloud's voice model on this device, beside the voice source selector: an
 * info button that opens a popover, so it works by tap and keyboard and also while WebGPU can't
 * be chosen. The parent opens it the first time WebGPU is chosen.
 */
import { formatDownloadSize, localSpeechModel } from '../../../shared/local-speech'

const open = defineModel<boolean>('open', { default: false })
const { t } = useI18n()
const size = formatDownloadSize(localSpeechModel.downloadBytes)
const topics = ['download', 'memory', 'power', 'slow', 'support', 'fallback'] as const
const headingId = useId()
</script>
<template>
  <UiPopover v-model:open="open">
    <UiPopoverTrigger as-child>
      <button
        type="button"
        class="local-speech-info inline-flex size-8 shrink-0 items-center justify-center rounded-full text-muted-foreground outline-none hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring data-[state=open]:bg-accent data-[state=open]:text-foreground [@media(pointer:coarse)]:size-11"
        :aria-label="t('readAloud.local.info.title')"
        :title="t('readAloud.local.info.title')"
      >
        <AppIcon
          name="info"
          :size="16"
        />
      </button>
    </UiPopoverTrigger>
    <UiPopoverContent
      class="local-speech-info-content w-[min(calc(340px*var(--interface-scale,1)),calc(100vw-16px))] max-h-[min(480px,70dvh)] overflow-y-auto p-3"
      align="end"
      :aria-labelledby="headingId"
    >
      <h4
        :id="headingId"
        class="mb-2 text-[length:var(--ui-small)] font-[600] text-foreground"
      >
        {{ t('readAloud.local.info.title') }}
      </h4>
      <p
        v-for="topic in topics"
        :key="topic"
        class="mb-2 text-[length:var(--ui-caption)] leading-[1.5] text-muted-foreground last:mb-0"
      >
        <strong class="font-[600] text-foreground">{{
          t(`readAloud.local.info.${topic}Title`)
        }}</strong>
        {{ t(`readAloud.local.info.${topic}`, { size }) }}
      </p>
    </UiPopoverContent>
  </UiPopover>
</template>
