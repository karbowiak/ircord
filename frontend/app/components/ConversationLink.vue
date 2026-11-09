<script setup lang="ts">
/**
 * A sidebar destination with shared unread, selected, muted and presence states. Rows that show
 * those states as dots, badges or opacity also name them for screen readers: `mentions` counts
 * mentions and `presence` describes a direct-message partner (see presenceLabel).
 */
const props = defineProps<{
  active?: boolean
  unread?: boolean
  direct?: boolean
  muted?: boolean
  mentions?: number
  presence?: string
}>()
const states = computed(() =>
  [
    props.presence,
    props.mentions ? countLabel(props.mentions, 'mention', 'mentions') : props.unread && 'unread',
    props.muted && 'muted',
  ].filter(Boolean),
)
</script>
<template>
  <button
    type="button"
    class="conversation-link flex w-full items-center gap-2 my-px mx-0 rounded-sm px-2 py-0 text-left text-[length:var(--ui-text)] [@media(pointer:coarse)]:select-none [@media(pointer:coarse)]:[-webkit-touch-callout:none]"
    :class="
      cn(
        direct
          ? 'is-direct min-h-[var(--row-height)]'
          : 'min-h-[calc(var(--row-height)_-_4px)] [@media(pointer:coarse)]:min-h-[44px]',
        active
          ? 'is-selected bg-selection text-foreground [&>svg]:text-primary [&_.presence]:border-selection'
          : 'text-muted-foreground hover:bg-accent hover:text-body [&>svg]:text-faint [&_.presence]:border-sidebar hover:[&_.presence]:border-accent',
        {
          'has-unread': unread,
          'font-semibold text-foreground': unread && !active,
          'is-muted': muted,
          'opacity-55': muted && !active,
        },
      )
    "
    :aria-current="active ? 'page' : undefined"
  >
    <slot /><span
      v-if="states.length"
      class="sr-only"
      >, {{ states.join(', ') }}</span
    >
  </button>
</template>
