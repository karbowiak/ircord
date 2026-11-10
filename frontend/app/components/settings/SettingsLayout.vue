<script setup vapor lang="ts">
/**
 * The frame of a settings-style dialog: a section list beside the content, with an optional
 * field above the list. On narrow screens the list becomes a row above the content.
 */
defineProps<{ label: string }>()
</script>
<template>
  <div
    class="settings-layout grid min-h-0 flex-1 [grid-template-areas:'search_content'_'nav_content'] [grid-template-columns:calc(200px_*_var(--interface-scale,1))_minmax(0,_1fr)] [grid-template-rows:auto_minmax(0,_1fr)] max-[700px]:[grid-template-areas:'search'_'nav'_'content'] max-[700px]:[grid-template-columns:1fr] max-[700px]:[grid-template-rows:auto_auto_minmax(0,_1fr)]"
  >
    <slot name="search" />
    <nav
      class="settings-nav flex flex-col gap-0.5 overflow-y-auto bg-sidebar px-2 py-3 [grid-area:nav] max-[700px]:flex-row max-[700px]:overflow-x-auto max-[700px]:p-2"
      :aria-label="label"
    >
      <slot name="nav" />
    </nav>
    <slot />
  </div>
</template>
