import type { VariantProps } from 'class-variance-authority'
import { cva } from 'class-variance-authority'

export { default as DropdownMenu } from './DropdownMenu.vue'
export { default as DropdownMenuContent } from './DropdownMenuContent.vue'
export { default as DropdownMenuItem } from './DropdownMenuItem.vue'
export { default as DropdownMenuLabel } from './DropdownMenuLabel.vue'
export { default as DropdownMenuSeparator } from './DropdownMenuSeparator.vue'
export { default as DropdownMenuSub } from './DropdownMenuSub.vue'
export { default as DropdownMenuSubContent } from './DropdownMenuSubContent.vue'
export { default as DropdownMenuSubTrigger } from './DropdownMenuSubTrigger.vue'
export { default as DropdownMenuTrigger } from './DropdownMenuTrigger.vue'

export const dropdownMenuContentClass =
  'z-60 min-w-50 max-w-70 max-h-[var(--reka-dropdown-menu-content-available-height)] overflow-y-auto rounded-md bg-rail p-1.5 shadow-popover outline-none'

export const dropdownMenuItemVariants = cva(
  'flex min-h-8 w-full cursor-default select-none items-center gap-[9px] rounded-sm px-2 py-1 text-left text-[length:var(--ui-small)] outline-none data-disabled:pointer-events-none data-disabled:opacity-45 [@media(pointer:coarse)]:min-h-[44px]',
  {
    variants: {
      variant: {
        default:
          'text-body [&>svg]:text-muted-foreground data-highlighted:bg-primary data-highlighted:text-primary-foreground data-highlighted:[&>svg]:text-inherit data-[state=open]:bg-primary data-[state=open]:text-primary-foreground data-[state=open]:[&>svg]:text-inherit',
        destructive:
          'text-destructive [&>svg]:text-destructive data-highlighted:bg-destructive data-highlighted:text-rail data-highlighted:[&>svg]:text-rail',
        // A compact square in a row of quick actions (reactions).
        quick:
          'h-10 flex-1 justify-center bg-secondary p-0 text-[20px] data-highlighted:bg-primary data-highlighted:text-primary-foreground',
      },
    },
    defaultVariants: { variant: 'default' },
  },
)
export type DropdownMenuItemVariants = VariantProps<typeof dropdownMenuItemVariants>
