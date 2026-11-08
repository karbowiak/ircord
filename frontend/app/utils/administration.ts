import type { InjectionKey } from 'vue'

/** Called by Administration pages when the backend refuses a request (403). */
export const administrationRefusedKey: InjectionKey<() => void> = Symbol('administration-refused')

export function useAdministrationRefusal() {
  const refused = inject(administrationRefusedKey, () => {})
  return (error: unknown) => {
    if (errorStatus(error) === 403) refused()
  }
}

const units = ['B', 'KB', 'MB', 'GB', 'TB']
export function formatAdminBytes(bytes: number) {
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  return `${value.toFixed(value < 10 && unit ? 1 : 0)} ${units[unit]}`
}

export function formatDuration(seconds: number) {
  const days = Math.floor(seconds / 86_400)
  const hours = Math.floor((seconds % 86_400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days) return `${days}d ${hours}h`
  if (hours) return `${hours}h ${minutes}m`
  return `${minutes}m`
}
