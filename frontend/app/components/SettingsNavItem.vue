<script setup lang="ts">
/**
 * One section in a SettingsLayout list. The active one is marked as the current page. A `danger`
 * item (Log out) sits apart at the end of the list.
 */
const props = defineProps<{ icon?: string; active?: boolean; danger?: boolean }>()
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
    :class="
      cn(
        'flex h-[34px] items-center gap-2.5 rounded-sm px-2.5 text-left text-[14px] text-muted-foreground hover:bg-accent hover:text-foreground max-[700px]:shrink-0',
        active && 'active bg-selection text-foreground [&_svg]:text-primary',
        danger &&
          'settings-logout mt-auto text-destructive hover:bg-destructive hover:text-rail focus-visible:bg-destructive focus-visible:text-rail max-[700px]:mt-0 max-[700px]:ml-auto',
      )
    "
    :aria-current="active ? 'page' : undefined"
  >
    <AppIcon
      v-if="icon"
      :name="icon"
      :size="17"
    /><slot />
  </button>
</template>
