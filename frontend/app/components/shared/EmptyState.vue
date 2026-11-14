<script setup vapor lang="ts">
import { cn } from '../../utils/cn'
import type { HTMLAttributes } from 'vue'
/**
 * Centred placeholder for an empty, loading or failed list: an icon in a tinted tile, an
 * optional headline, a line and an optional action. Slotted content may lead with its own icon,
 * which gets the same tile.
 */
const props = withDefaults(
  defineProps<{ as?: string; icon?: string; title?: string; class?: HTMLAttributes['class'] }>(),
  {
    as: 'div',
    icon: undefined,
    title: undefined,
    class: undefined,
  },
)
</script>
<template>
  <component
    :is="as"
    :class="
      cn(
        'search-empty flex flex-col items-center gap-3 px-4 py-14 text-center text-[length:var(--ui-small)] leading-relaxed text-muted-foreground [&_p]:m-0 [&_p]:max-w-[52ch] [&>svg:first-child]:mb-1 [&>svg:first-child]:box-content [&>svg:first-child]:rounded-2xl [&>svg:first-child]:p-4 [&>svg:first-child]:text-[var(--accent)] [&>svg:first-child]:[background:color-mix(in_srgb,var(--accent)_12%,transparent)] [&>svg:first-child]:shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--accent)_22%,transparent),0_10px_30px_color-mix(in_srgb,var(--accent)_14%,transparent)]',
        props.class,
      )
    "
  >
    <AppIcon
      v-if="icon"
      :name="icon"
      :size="26"
    />
    <h2
      v-if="title"
      class="m-0 -mb-1.5 text-[length:var(--ui-heading)] leading-tight font-[700] tracking-[-0.01em] text-foreground"
    >
      {{ title }}
    </h2>
    <slot />
  </component>
</template>
