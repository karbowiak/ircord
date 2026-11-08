export { default as Select } from './Select.vue'
export { default as SelectContent } from './SelectContent.vue'
export { default as SelectItem } from './SelectItem.vue'
export { default as SelectTrigger } from './SelectTrigger.vue'

/** The closed select's look, shared with other buttons that open a choice. */
export const selectTriggerClass =
  'group inline-flex min-h-9 w-full items-center justify-between gap-2 rounded-md border border-solid border-border bg-field py-1.5 pr-2 pl-2.5 text-left text-[13px] leading-[1.5] text-foreground outline-none hover:not-disabled:border-[color-mix(in_srgb,var(--muted)_45%,var(--border))] focus-visible:border-[color-mix(in_srgb,var(--accent)_60%,var(--border))] data-[state=open]:border-[color-mix(in_srgb,var(--accent)_60%,var(--border))] aria-invalid:border-destructive disabled:cursor-not-allowed disabled:opacity-55 data-placeholder:text-faint'
