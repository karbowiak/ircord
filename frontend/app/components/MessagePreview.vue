<script setup vapor lang="ts">
/** Two sample messages in the current theme, layout, font, clock and name colours. */
import { memberNameColor } from '../../shared/utils/identity'
const { locale, t: $t } = useI18n()
const appearance = useAppearanceStore()
const settings = computed(() => appearance.settings)
const sampleName = (id: string) => ({
  color: settings.value.coloredNames
    ? memberNameColor({ id, account: id }, appearance.mode, appearance.tokens.main)
    : 'var(--text)',
})
const sampleTime = (minutes: number) =>
  new Date(2026, 0, 1, 9, minutes).toLocaleTimeString(locale.value, {
    hour: '2-digit',
    minute: '2-digit',
    hour12: settings.value.timeFormat === '12h',
  })
</script>
<template>
  <div
    class="appearance-preview sticky top-[-20px] z-2 py-3.5 px-4 rounded-md bg-background shadow-[inset_0_0_0_1px_var(--border),_0_8px_16px_-12px_rgb(0_0_0_/_50%)]"
    aria-hidden="true"
  >
    <div
      class="preview-row grid gap-[var(--group-gap)]"
      :data-layout="settings.messageLayout"
    >
      <template v-if="settings.messageLayout === 'irc'">
        <span
          class="preview-line font-chat leading-[1.5] text-body grid [grid-template-columns:auto_100px_1fr] gap-2.5 items-baseline"
          ><time>{{ sampleTime(41) }}</time
          ><b :style="sampleName('jules')">jules</b
          ><span>{{ $t('workspaceUi.preview.review') }}</span></span
        >
        <span
          class="preview-line font-chat leading-[1.5] text-body grid [grid-template-columns:auto_100px_1fr] gap-2.5 items-baseline"
          ><time>{{ sampleTime(42) }}</time
          ><b :style="sampleName('noah')">noah</b
          ><i18n-t
            keypath="workspaceUi.preview.link"
            tag="span"
            ><template #channel><a>#development</a></template></i18n-t
          ></span
        >
      </template>
      <template v-else>
        <span class="preview-message grid [grid-template-columns:40px_1fr] gap-3.5"
          ><i
            class="preview-avatar tone-mint bg-[#b8d6c4] text-[#355944] w-10 h-10 rounded-full flex items-center justify-center not-italic font-[600]"
            >J</i
          ><span
            ><span class="preview-meta flex items-baseline gap-2"
              ><b :style="sampleName('jules')">Jules</b><time>{{ sampleTime(41) }}</time></span
            ><span class="preview-text block font-chat leading-[1.5] text-body">{{
              $t('workspaceUi.preview.review')
            }}</span></span
          ></span
        >
        <span class="preview-message grid [grid-template-columns:40px_1fr] gap-3.5"
          ><i
            class="preview-avatar tone-blue bg-[#aabfd9] text-[#354a65] w-10 h-10 rounded-full flex items-center justify-center not-italic font-[600]"
            >N</i
          ><span
            ><span class="preview-meta flex items-baseline gap-2"
              ><b :style="sampleName('noah')">Noah</b><time>{{ sampleTime(42) }}</time></span
            ><span class="preview-text block font-chat leading-[1.5] text-body"
              ><i18n-t
                keypath="workspaceUi.preview.link"
                tag="span"
                ><template #channel><a>#development</a></template></i18n-t
              ></span
            ></span
          ></span
        >
      </template>
    </div>
  </div>
</template>
