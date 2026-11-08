<script setup lang="ts">
/**
 * One setting: its name and description beside (or above, when `stacked`) its control. Give it
 * `data-setting` with the setting's id from shared/settings-index.ts. AppSwitch and AppSegmented
 * inside it are described by the row's description automatically.
 */
const props = withDefaults(
  defineProps<{ label: string; description?: string; stacked?: boolean }>(),
  { description: undefined, stacked: false },
)
const slots = useSlots()
const descriptionId = useId()
provide(
  settingDescriptionKey,
  computed(() => (props.description || slots.description ? descriptionId : undefined)),
)
</script>
<template>
  <div
    class="setting-row"
    :class="{ stacked }"
  >
    <span class="setting-copy"
      ><strong>{{ label }}</strong
      ><small
        v-if="description || $slots.description"
        :id="descriptionId"
        ><slot name="description">{{ description }}</slot></small
      ></span
    >
    <slot />
  </div>
</template>
