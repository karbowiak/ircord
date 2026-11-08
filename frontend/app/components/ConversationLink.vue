<script setup lang="ts">
/** A sidebar destination with shared unread, selected, muted and presence states. */
defineProps<{ active?: boolean; unread?: boolean; direct?: boolean; muted?: boolean }>()
</script>
<template>
  <button
    type="button"
    class="conversation-link flex w-full items-center gap-2 my-px mx-0 rounded-sm px-2 py-0 text-left text-[14px] [@media(pointer:coarse)]:select-none [@media(pointer:coarse)]:[-webkit-touch-callout:none]"
    :class="
      cn(
        direct ? 'is-direct min-h-[calc(var(--row-height)_+_4px)]' : 'min-h-[var(--row-height)]',
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
    <slot />
  </button>
</template>
