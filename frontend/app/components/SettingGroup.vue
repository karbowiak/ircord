<script setup lang="ts">
/** A titled group of settings, with an optional note on where its values are kept. */
defineProps<{ title: string; scope?: string }>()
const headingId = useId()
</script>
<template>
  <section
    class="setting-group"
    :aria-labelledby="headingId"
  >
    <header class="setting-group-heading">
      <h3 :id="headingId">{{ title }}</h3>
      <small
        v-if="scope || $slots.scope"
        class="setting-scope"
        ><slot name="scope">{{ scope }}</slot></small
      >
    </header>
    <slot />
  </section>
</template>
