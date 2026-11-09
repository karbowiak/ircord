<script setup lang="ts">
/** A titled group of settings, with an optional note on where its values are kept. */
defineProps<{ title: string; scope?: string }>()
const headingId = useId()
</script>
<template>
  <section
    class="setting-group grid gap-3.5"
    :aria-labelledby="headingId"
  >
    <header
      class="setting-group-heading flex items-baseline justify-between flex-wrap gap-[4px_12px] pb-2 border-b [border-bottom-style:solid] border-b-border"
    >
      <h3 :id="headingId">{{ title }}</h3>
      <small
        v-if="scope || $slots.scope"
        class="setting-scope text-[length:var(--ui-caption)] text-faint"
        ><slot name="scope">{{ scope }}</slot></small
      >
    </header>
    <slot />
  </section>
</template>
