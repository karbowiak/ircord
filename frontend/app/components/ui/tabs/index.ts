import type { VariantProps } from 'class-variance-authority'
import { cva } from 'class-variance-authority'

export { default as Tabs } from './Tabs.vue'
export { default as TabsContent } from './TabsContent.vue'
export { default as TabsList } from './TabsList.vue'
export { default as TabsTrigger } from './TabsTrigger.vue'

export const tabsTriggerVariants = cva(
  'inline-flex flex-none items-center whitespace-nowrap text-[length:var(--ui-small)] text-muted-foreground hover:text-foreground disabled:cursor-not-allowed disabled:opacity-45 [@media(pointer:coarse)]:min-h-[44px]',
  {
    variants: {
      variant: {
        // A dialog's tab row, with the active tab underlined against the row's bottom border.
        underline:
          'relative pb-[9px] font-[550] data-[state=active]:text-foreground data-[state=active]:after:absolute data-[state=active]:after:inset-x-0 data-[state=active]:after:-bottom-px data-[state=active]:after:h-0.5 data-[state=active]:after:rounded-[2px] data-[state=active]:after:bg-primary [&>span]:ms-1 [&>span]:text-[12px] [&>span]:text-faint',
        pill: 'gap-1.5 rounded-sm px-2.5 py-[7px] hover:bg-accent data-[state=active]:bg-selection data-[state=active]:text-primary',
      },
    },
    defaultVariants: { variant: 'underline' },
  },
)
export type TabsTriggerVariants = VariantProps<typeof tabsTriggerVariants>
