import type { VariantProps } from 'class-variance-authority'
import { cva } from 'class-variance-authority'

export { default as Alert } from './Alert.vue'

export const alertVariants = cva('relative text-[13px]', {
  variants: {
    variant: {
      default: 'text-card-foreground',
      destructive: 'text-destructive',
    },
    presentation: {
      panel: 'w-full rounded-md border border-solid border-border bg-card px-4 py-3',
      inline: 'block bg-transparent p-0',
    },
  },
  defaultVariants: { variant: 'default', presentation: 'panel' },
})

export type AlertVariants = VariantProps<typeof alertVariants>
