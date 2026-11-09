/** Time and day labels that follow the user's 12/24-hour preference. */
export function useTimeFormat() {
  const appearance = useAppearanceStore()
  const { locale, t } = useI18n()
  const time = (iso: string) =>
    new Date(iso).toLocaleTimeString(locale.value, {
      hour: '2-digit',
      minute: '2-digit',
      hour12: appearance.settings.timeFormat === '12h',
    })
  const full = (iso: string) =>
    new Date(iso).toLocaleString(locale.value, { hour12: appearance.settings.timeFormat === '12h' })
  const day = (iso: string) => {
    const date = new Date(iso)
    const today = new Date()
    const yesterday = new Date(today.getTime() - 86_400_000)
    if (date.toDateString() === today.toDateString()) return t('dates.today')
    if (date.toDateString() === yesterday.toDateString()) return t('dates.yesterday')
    return date.toLocaleDateString(locale.value, {
      weekday: 'long',
      month: 'long',
      day: 'numeric',
      ...(date.getFullYear() !== today.getFullYear() ? { year: 'numeric' } : {}),
    })
  }
  return { time, full, day }
}
