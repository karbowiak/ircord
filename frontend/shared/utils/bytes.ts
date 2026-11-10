const units = ['B', 'KB', 'MB', 'GB', 'TB'] as const

function scaleBytes(bytes: number, maxUnit: number) {
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < maxUnit) {
    value /= 1024
    unit++
  }
  return { value, unit }
}

/** File sizes stay in MB above that threshold, matching upload limits and attachments. */
export function formatBytes(bytes: number, locale = 'en') {
  const scaled = scaleBytes(bytes, 2)
  // The previous file formatter selected MB for NaN because both comparisons were false.
  const unit = Number.isNaN(bytes) ? 2 : scaled.unit
  const digits = unit === 2 && bytes < 10 * 1024 * 1024 ? 1 : 0
  const value = new Intl.NumberFormat(locale, { maximumFractionDigits: digits }).format(
    scaled.value,
  )
  return `${value} ${units[unit]}`
}

type NumberFormatter = (value: number, options: { maximumFractionDigits: number }) => string

/** Overview uses one fractional digit only for non-byte values below ten. */
export function formatOverviewBytes(bytes: number, format: NumberFormatter) {
  const { value, unit } = scaleBytes(bytes, units.length - 1)
  const maximumFractionDigits = value < 10 && unit ? 1 : 0
  return `${format(value, { maximumFractionDigits })} ${units[unit]}`
}

/** Database measurements allow one fractional digit for every non-byte unit. */
export function formatDatabaseBytes(bytes: number, format: NumberFormatter) {
  const { value, unit } = scaleBytes(bytes, units.length - 1)
  return `${format(value, { maximumFractionDigits: unit ? 1 : 0 })} ${units[unit]}`
}
