<script setup lang="ts">
/** One section in a SettingsLayout list. The active one is marked as the current page. */
const props = defineProps<{ icon?: string; active?: boolean }>()
const el = ref<HTMLButtonElement | null>(null)
// On narrow screens the list scrolls sideways; keep the chosen section in view.
watch(
  () => props.active,
  (active) =>
    active && nextTick(() => el.value?.scrollIntoView({ block: 'nearest', inline: 'nearest' })),
  { immediate: true },
)
</script>
<template>
  <button
    ref="el"
    type="button"
    class="flex h-[34px] items-center gap-2.5 rounded-sm px-2.5 text-left text-[14px] text-muted-foreground hover:bg-accent hover:text-foreground max-[700px]:shrink-0 [&.active]:bg-selection [&.active]:text-foreground [&.active_svg]:text-primary"
    :class="{ active }"
    :aria-current="active ? 'page' : undefined"
  >
    <AppIcon
      v-if="icon"
      :name="icon"
      :size="17"
    /><slot />
  </button>
</template>
