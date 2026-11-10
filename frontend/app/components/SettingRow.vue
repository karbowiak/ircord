<script setup vapor lang="ts">
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
    :class="
      cn(
        'setting-row flex items-center justify-between gap-4',
        stacked && 'stacked flex-col items-stretch gap-2.5',
      )
    "
  >
    <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
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
