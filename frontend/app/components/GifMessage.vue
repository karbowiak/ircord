<script setup lang="ts">
import type { GifItem } from '../../shared/types/media'
const props = defineProps<{ gif: GifItem }>()
const emit = defineEmits<{ image: [url: string, alt: string] }>()
const { playing, handlers, claimTap } = useMediaPlayback()
const library = useLibraryStore()
const failed = ref(false)
const loaded = ref(false)
const saved = computed(() => library.isFavoriteGif(props.gif))
watch(
  () => props.gif.url,
  () => {
    failed.value = false
    loaded.value = false
  },
)
function open() {
  if (!claimTap()) emit('image', props.gif.url, props.gif.title)
}
</script>
<template>
  <div class="message-gif">
    <a
      v-if="failed"
      :href="gif.url"
      target="_blank"
      rel="noopener noreferrer"
      >{{ gif.title }} (GIF)</a
    >
    <template v-else>
      <button
        class="gif-message-image media-frame"
        :class="{ 'is-loading': !loaded, 'is-playing': playing }"
        :aria-label="`View ${gif.title} GIF`"
        :aria-busy="!loaded"
        :style="{ aspectRatio: `${gif.width} / ${gif.height}` }"
        v-on="handlers"
        @click="open"
      >
        <img
          :src="playing ? gif.previewUrl : gif.stillUrl"
          :alt="gif.title"
          :width="gif.width"
          :height="gif.height"
          decoding="async"
          referrerpolicy="no-referrer"
          @load="loaded = true"
          @error="failed = true"
        />
        <span
          class="media-kind"
          aria-hidden="true"
          >GIF</span
        >
      </button>
      <button
        class="gif-message-favorite icon-button"
        :class="{ 'is-favorite': saved }"
        :aria-label="saved ? 'Remove GIF from favorites' : 'Save GIF to favorites'"
        :title="saved ? 'Remove from favorites' : 'Add to favorites'"
        :aria-pressed="saved"
        @click="library.toggleGif(gif)"
      >
        <AppIcon
          name="star"
          :size="15"
        />
      </button>
      <a
        class="gif-attribution"
        :href="gif.url"
        target="_blank"
        rel="noopener noreferrer"
        >KLIPY</a
      >
    </template>
  </div>
</template>
