import type { VariantProps } from 'class-variance-authority'
import { cva } from 'class-variance-authority'

export { default as Button } from './Button.vue'

export const buttonVariants = cva(
  'inline-flex items-center justify-center gap-[7px] whitespace-nowrap rounded-md text-[13px] font-[550] transition-[background,filter] duration-[120ms] disabled:cursor-not-allowed disabled:opacity-45 [&_svg]:pointer-events-none [&_svg]:shrink-0 aria-invalid:border-destructive',
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground hover:not-disabled:brightness-[1.07]',
        destructive:
          'bg-destructive text-destructive-foreground hover:not-disabled:brightness-[1.07]',
        outline:
          'border bg-background shadow-xs hover:bg-accent hover:text-accent-foreground dark:bg-input/30 dark:border-input dark:hover:bg-input/50',
        secondary:
          'bg-secondary text-secondary-foreground shadow-[inset_0_0_0_1px_var(--border)] hover:not-disabled:bg-accent',
        // A secondary button for a destructive action: quiet until it matters.
        'secondary-destructive':
          'bg-secondary text-destructive shadow-[inset_0_0_0_1px_var(--border)] hover:not-disabled:bg-accent',
        ghost:
          'text-muted-foreground hover:not-disabled:bg-accent hover:not-disabled:text-foreground',
        link: 'text-primary hover:not-disabled:bg-selection',
        inline: 'text-primary underline-offset-2 hover:not-disabled:underline',
      },
      size: {
        default: 'button h-9 px-[14px]',
        xs: "button h-6 gap-1 rounded-md px-2 text-xs has-[>svg]:px-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: 'button h-[30px] px-[11px] text-[12px]',
        lg: 'button h-10 rounded-md px-6 has-[>svg]:px-4',
        icon: 'icon-button size-9 shrink-0 p-0 [@media(pointer:coarse)]:min-w-10 [@media(pointer:coarse)]:min-h-10',
        'icon-xs':
          "icon-button size-6 shrink-0 p-0 rounded-md [&_svg:not([class*='size-'])]:size-3 [@media(pointer:coarse)]:min-w-10 [@media(pointer:coarse)]:min-h-10",
        'icon-sm':
          'icon-button size-[30px] shrink-0 p-0 rounded-sm transition-[color,background] [@media(pointer:coarse)]:min-w-10 [@media(pointer:coarse)]:min-h-10',
        'icon-lg':
          'icon-button size-10 shrink-0 p-0 [@media(pointer:coarse)]:min-w-10 [@media(pointer:coarse)]:min-h-10',
        text: 'text-button h-auto rounded-sm px-2 py-1 text-[13px] font-normal whitespace-normal',
        // A link-like action inside running text: no box, the surrounding type.
        inline:
          'inline h-auto rounded-none p-0 align-baseline font-normal text-[length:inherit] whitespace-normal',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)
export type ButtonVariants = VariantProps<typeof buttonVariants>
