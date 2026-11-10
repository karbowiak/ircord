<script setup vapor lang="ts">
/**
 * A link to another section or setting (an id from shared/settings-index.ts), for preferences
 * another section owns. It opens through the same path as settings search.
 */
import {
  sectionName,
  settingEntry,
  settingsSections,
  settingText,
} from '../../shared/settings-index'
const props = defineProps<{ to: string }>()
const go = useSettingsNavigation()
const { t } = useI18n()
const messages = useSettingsMessages()
const name = computed(() => {
  const section = settingsSections.find((s) => s.id === props.to)
  if (section) return sectionName(section.id, t)
  const entry = settingEntry(props.to)
  return entry ? settingText(messages, entry).label : props.to
})
</script>
<template>
  <button
    type="button"
    class="setting-link inline text-primary [font-size:inherit] underline [text-underline-offset:2px]"
    @click="go(to)"
  >
    <slot>{{ name }}</slot>
  </button>
</template>
