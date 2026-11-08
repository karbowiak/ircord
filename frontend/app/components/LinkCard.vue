<script setup lang="ts">
import type { components } from '../../shared/generated/api'
import { publicMediaURL } from '../../shared/utils/link-media'
/**
 * A page summary drawn by the app. Its height follows from the card's data and width alone: the
 * image box has a fixed shape and the title and description areas a fixed number of lines, so the
 * card never changes size after it appears (an image that fails keeps its box).
 */
const props = defineProps<{ card: components['schemas']['LinkMediaCard']; url: string }>()
const href = computed(() => publicMediaURL(props.url))
const image = computed(() => props.card.image && publicMediaURL(props.card.image))
const icon = computed(() => props.card.icon && publicMediaURL(props.card.icon))
const imageFailed = ref(false)
const iconFailed = ref(false)
</script>
<template>
  <a class="link-card" :href="href" target="_blank" rel="noopener noreferrer">
    <span v-if="image" class="link-card-image" :class="{ 'is-failed': imageFailed }">
      <img v-if="!imageFailed" :src="image" alt="" loading="lazy" decoding="async" referrerpolicy="no-referrer" @error="imageFailed = true" />
    </span>
    <span class="link-card-text">
      <span class="link-card-site">
        <img v-if="icon && !iconFailed" :src="icon" alt="" width="16" height="16" referrerpolicy="no-referrer" @error="iconFailed = true" />
        <span>{{ card.site }}</span>
      </span>
      <strong class="link-card-title">{{ card.title }}</strong>
      <span v-if="card.description" class="link-card-description">{{ card.description }}</span>
    </span>
  </a>
</template>
