<script setup lang="ts">
const props = defineProps<{ src: string; label: string; playing?: boolean }>()
const failed = ref(false)
const imageSource = computed(() => {
  if (props.playing) return props.src
  return `${props.src}${props.src.includes('?') ? '&' : '?'}static=true`
})
watch(imageSource, () => {
  failed.value = false
})
</script>
<template>
  <span class="emote-still">
    <img
      v-if="!failed"
      :src="imageSource"
      :alt="label"
      loading="lazy"
      decoding="async"
      @error="failed = true"
    />
    <span
      v-else
      class="emote-fallback"
      role="img"
      :aria-label="label"
      >:{{ label }}:</span
    >
  </span>
</template>
<style scoped>
.emote-still {
  display: inline-block;
  overflow: hidden;
}
.emote-still img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.emote-fallback {
  display: block;
  width: 100%;
  height: 100%;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
</style>
