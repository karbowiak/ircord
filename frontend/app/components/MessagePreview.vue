<script setup lang="ts">
/** Two sample messages in the current theme, layout, font, clock and name colours. */
import { memberNameColor } from '../../shared/utils/identity'
const appearance = useAppearanceStore()
const settings = computed(() => appearance.settings)
const sampleName = (id: string) => ({
  color: settings.value.coloredNames
    ? memberNameColor({ id, account: id }, appearance.mode, appearance.tokens.main)
    : 'var(--text)',
})
const sampleTime = (minutes: number) =>
  new Date(2026, 0, 1, 9, minutes).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
    hour12: settings.value.timeFormat === '12h',
  })
</script>
<template>
  <div
    class="appearance-preview"
    aria-hidden="true"
  >
    <div
      class="preview-row"
      :data-layout="settings.messageLayout"
    >
      <template v-if="settings.messageLayout === 'irc'">
        <span class="preview-line"
          ><time>{{ sampleTime(41) }}</time
          ><b :style="sampleName('jules')">jules</b><span>anyone up for a code review?</span></span
        >
        <span class="preview-line"
          ><time>{{ sampleTime(42) }}</time
          ><b :style="sampleName('noah')">noah</b
          ><span>sure, send the link <a>#development</a></span></span
        >
      </template>
      <template v-else>
        <span class="preview-message"
          ><i class="preview-avatar tone-mint">J</i
          ><span
            ><span class="preview-meta"
              ><b :style="sampleName('jules')">Jules</b><time>{{ sampleTime(41) }}</time></span
            ><span class="preview-text">anyone up for a code review?</span></span
          ></span
        >
        <span class="preview-message"
          ><i class="preview-avatar tone-blue">N</i
          ><span
            ><span class="preview-meta"
              ><b :style="sampleName('noah')">Noah</b><time>{{ sampleTime(42) }}</time></span
            ><span class="preview-text">sure, send the link <a>#development</a></span></span
          ></span
        >
      </template>
    </div>
  </div>
</template>
