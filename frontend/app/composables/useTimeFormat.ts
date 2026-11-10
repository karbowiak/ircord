import { formatDate } from '../utils/time-format'

/** Time and day labels that follow the user's 12/24-hour preference. */
export function useTimeFormat() {
  const appearance = useAppearanceStore()
  const { locale, t } = useI18n()
  const time = (iso: string) =>
    formatDate(new Date(iso), locale.value, 'time', appearance.settings.timeFormat === '12h')
  const full = (iso: string) =>
    formatDate(new Date(iso), locale.value, 'full', appearance.settings.timeFormat === '12h')
  const day = (iso: string) => {
    const date = new Date(iso)
    const today = new Date()
    const yesterday = new Date(today)
    yesterday.setDate(today.getDate() - 1)
    if (date.toDateString() === today.toDateString()) return t('dates.today')
    if (date.toDateString() === yesterday.toDateString()) return t('dates.yesterday')
    return formatDate(
      date,
      locale.value,
      date.getFullYear() !== today.getFullYear() ? 'dayWithYear' : 'day',
    )
  }
  return { time, full, day }
}
