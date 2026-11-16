/**
 * Shared `Intl` formatters. Creating one costs far more than using it, and lists that re-sort or
 * rows that relabel on every chat event would otherwise create one per comparison or per row.
 */
const collators = new Map<string, Intl.Collator>()
const listFormats = new Map<string, Intl.ListFormat>()

/** One collator for each locale and set of options. */
export function sharedCollator(locale?: string, options?: Intl.CollatorOptions) {
  const key = JSON.stringify([locale ?? null, options ?? null])
  let collator = collators.get(key)
  if (!collator) collators.set(key, (collator = new Intl.Collator(locale, options)))
  return collator
}

const numberFormats = new Map<string, Intl.NumberFormat>()

/** One number format for each locale and set of options. */
export function sharedNumberFormat(locale?: string, options?: Intl.NumberFormatOptions) {
  const key = JSON.stringify([locale ?? null, options ?? null])
  let format = numberFormats.get(key)
  if (!format) numberFormats.set(key, (format = new Intl.NumberFormat(locale, options)))
  return format
}

/** One list format for each locale and set of options. */
export function sharedListFormat(locale?: string, options?: Intl.ListFormatOptions) {
  const key = JSON.stringify([locale ?? null, options ?? null])
  let format = listFormats.get(key)
  if (!format) listFormats.set(key, (format = new Intl.ListFormat(locale, options)))
  return format
}
