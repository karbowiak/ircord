type DateStyle = 'time' | 'full' | 'day' | 'dayWithYear'

// Intl formatters are expensive native objects. Share a bounded set across rows,
// rather than allocating new ones on every timestamp render or menu update.
const formatters = new Map<string, Intl.DateTimeFormat>()
const options: Record<DateStyle, Intl.DateTimeFormatOptions> = {
  time: { hour: '2-digit', minute: '2-digit' },
  full: {
    year: 'numeric',
    month: 'numeric',
    day: 'numeric',
    hour: 'numeric',
    minute: 'numeric',
    second: 'numeric',
  },
  day: { weekday: 'long', month: 'long', day: 'numeric' },
  dayWithYear: { weekday: 'long', month: 'long', day: 'numeric', year: 'numeric' },
}

export function formatDate(date: Date, locale: string, style: DateStyle, hour12 = false) {
  if (Number.isNaN(date.getTime())) return date.toString()
  const key = JSON.stringify([locale, style, hour12])
  let formatter = formatters.get(key)
  if (!formatter) {
    formatter = new Intl.DateTimeFormat(locale, {
      ...options[style],
      ...(style === 'time' || style === 'full' ? { hour12 } : {}),
    })
    if (formatters.size >= 32) formatters.delete(formatters.keys().next().value!)
    formatters.set(key, formatter)
  }
  return formatter.format(date)
}
