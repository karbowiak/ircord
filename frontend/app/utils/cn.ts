import { clsx, type ClassValue } from 'clsx'
import { extendTailwindMerge } from 'tailwind-merge'

// Font sizes in the migration are independent of line-height. Preserve an explicit leading
// value when a caller overrides only the size; slash modifiers still replace both together.
const merge = extendTailwindMerge({ override: { conflictingClassGroups: { 'font-size': [] } } })

/** Merge utility overrides without losing conditional classes or application hooks. */
export function cn(...inputs: ClassValue[]) {
  return merge(clsx(inputs))
}
