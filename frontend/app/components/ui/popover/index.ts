export { default as Popover } from './Popover.vue'
export { default as PopoverContent } from './PopoverContent.vue'
export { default as PopoverTrigger } from './PopoverTrigger.vue'

/** Matches menus (dropdownMenuContentClass), so popovers and menus read as one family. Above
 * dialogs (z-70), so a popover opened from a dialog shows over it. */
export const popoverContentClass =
  'z-[80] w-72 max-w-[calc(100vw_-_16px)] rounded-md bg-rail p-1.5 text-foreground shadow-popover outline-none'
