<script setup lang="ts">
/**
 * A link to another section or setting (an id from shared/settings-index.ts), for preferences
 * another section owns. It opens through the same path as settings search.
 */
import { sectionName, settingEntry, settingsSections } from '../../shared/settings-index'
const props = defineProps<{ to: string }>()
const go = useSettingsNavigation()
const name = computed(() => {
  const section = settingsSections.find((s) => s.id === props.to)
  return section ? sectionName(section.id) : (settingEntry(props.to)?.label ?? props.to)
})
</script>
<template>
  <button
    type="button"
    class="setting-link"
    @click="go(to)"
  >
    <slot>{{ name }}</slot>
  </button>
</template>
