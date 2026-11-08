<script setup lang="ts">
/**
 * A linked audio file: the browser's own player, which loads nothing until someone presses play
 * (audio never plays by itself, whatever the autoplay setting) and falls back to a link.
 */
const props = defineProps<{ src: string; host: string }>()
const failed = ref(false)
const name = computed(() => {
  try {
    return decodeURIComponent(new URL(props.src).pathname.split('/').pop() || 'Audio')
  } catch {
    return 'Audio'
  }
})
watch(
  () => props.src,
  () => (failed.value = false),
)
</script>
<template>
  <a
    v-if="failed"
    class="media-unavailable"
    :href="src"
    target="_blank"
    rel="noopener noreferrer"
    >Audio unavailable · {{ host }}</a
  >
  <figure
    v-else
    class="message-audio"
  >
    <figcaption>
      <AppIcon
        name="audio"
        :size="18"
      /><a
        :href="src"
        target="_blank"
        rel="noopener noreferrer"
        ><strong>{{ name }}</strong></a
      ><small>{{ host }}</small>
    </figcaption>
    <audio
      :src="src"
      controls
      preload="none"
      referrerpolicy="no-referrer"
      :aria-label="`Audio ${name} from ${host}`"
      @error="failed = true"
    />
  </figure>
</template>
