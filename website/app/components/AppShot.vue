<script setup lang="ts">
import shots from '../shots.json'

/**
 * A screenshot from scripts/capture.mjs in the visitor's light or dark mode. Screenshots are
 * taken at twice their display size; the width and height reserve the space before they load.
 */
const props = defineProps<{
  name: keyof typeof shots
  alt: string
  /** Load at once instead of when scrolled near. */
  eager?: boolean
  /** Classes for the image, such as a crop; by default it keeps its own proportions. */
  imgClass?: string
}>()
const size = computed(() => shots[props.name])
</script>

<template>
  <picture class="block h-full">
    <source
      :srcset="`/shots/${name}-dark.webp`"
      media="(prefers-color-scheme: dark)"
      type="image/webp"
    />
    <img
      :src="`/shots/${name}-light.webp`"
      :width="size.width / 2"
      :height="size.height / 2"
      :alt="alt"
      :loading="eager ? 'eager' : 'lazy'"
      decoding="async"
      :class="imgClass ?? 'block h-auto w-full'"
    />
  </picture>
</template>
