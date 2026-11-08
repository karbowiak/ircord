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
  <a
    class="link-card block overflow-hidden text-foreground bg-secondary border border-solid border-border rounded-[12px] no-underline"
    :href="href"
    target="_blank"
    rel="noopener noreferrer"
  >
    <span
      v-if="image"
      class="link-card-image block [aspect-ratio:1.91] bg-card border-b [border-bottom-style:solid] border-b-border"
      :class="{ 'is-failed': imageFailed }"
    >
      <img
        v-if="!imageFailed"
        :src="image"
        alt=""
        loading="lazy"
        decoding="async"
        referrerpolicy="no-referrer"
        @error="imageFailed = true"
      />
    </span>
    <span class="link-card-text grid gap-1 pt-2.5 pr-3 pb-3 pl-3">
      <span
        class="link-card-site flex items-center gap-1.5 h-4 overflow-hidden text-muted-foreground text-[12px] leading-[16px] whitespace-nowrap text-ellipsis"
      >
        <img
          v-if="icon && !iconFailed"
          :src="icon"
          alt=""
          width="16"
          height="16"
          referrerpolicy="no-referrer"
          @error="iconFailed = true"
        />
        <span>{{ card.site }}</span>
      </span>
      <strong
        class="link-card-title [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2] overflow-hidden wrap-anywhere h-10 text-[14px] font-[600] leading-[20px]"
        >{{ card.title }}</strong
      >
      <span
        v-if="card.description"
        class="link-card-description [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2] overflow-hidden wrap-anywhere h-9.5 text-body text-[13px] leading-[19px]"
        >{{ card.description }}</span
      >
    </span>
  </a>
</template>
