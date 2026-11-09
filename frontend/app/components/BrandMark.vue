<script setup lang="ts">
// IRCord's own brand mark: the favicon itself, so every branding surface shows the same artwork.
withDefaults(defineProps<{ size?: number }>(), { size: 48 })
</script>
<template>
  <img
    src="/favicon.svg"
    alt=""
    :width="size"
    :height="size"
    decoding="async"
    draggable="false"
  >
</template>
