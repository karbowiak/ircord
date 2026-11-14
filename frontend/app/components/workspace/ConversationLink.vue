<script setup vapor lang="ts">
import { cn } from '../../utils/cn'
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
  pill?: boolean
}>()
const { t, n } = useI18n()
const states = computed(() =>
  [
    props.presence,
    props.mentions
      ? t('workspaceUi.sidebar.mentionCount', { count: n(props.mentions) }, props.mentions)
      : props.unread && t('workspaceUi.sidebar.unread'),
    props.muted && t('workspaceUi.sidebar.muted'),
  ].filter(Boolean),
)
</script>
<template>
  <button
    type="button"
    class="conversation-link flex w-full items-center gap-2 my-px mx-0 py-0 text-left text-[length:var(--ui-text)] transition-colors [@media(pointer:coarse)]:select-none [@media(pointer:coarse)]:[-webkit-touch-callout:none]"
    :class="
      cn(
        pill ? 'rounded-full px-3' : 'rounded-md px-2',
        direct
          ? 'is-direct min-h-[var(--row-height)]'
          : 'min-h-[calc(var(--row-height)_-_4px)] [@media(pointer:coarse)]:min-h-[44px]',
        active
          ? 'is-selected [background:linear-gradient(90deg,color-mix(in_srgb,var(--accent)_22%,var(--sidebar)),color-mix(in_srgb,var(--accent)_6%,var(--sidebar)))] shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--accent)_32%,transparent)] text-foreground [&>svg]:text-primary [&_.presence]:border-selection'
          : pill
            ? 'bg-field/80 text-muted-foreground shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_75%,transparent)] hover:bg-accent hover:text-body [&>svg]:text-faint [&_.presence]:border-sidebar hover:[&_.presence]:border-accent'
            : 'text-muted-foreground hover:bg-accent hover:text-body [&>svg]:text-faint [&_.presence]:border-sidebar hover:[&_.presence]:border-accent',
        {
          'has-unread': unread,
          'font-semibold text-foreground': unread && !active,
          'is-muted': muted,
          // Quieter than other rows, but still readable (4.5:1): the faint colour, not opacity.
          'text-faint': muted && !active,
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
