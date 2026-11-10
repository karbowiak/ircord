import type { InjectionKey } from 'vue'

/** Called by Administration pages when the backend refuses a request (403). */
export const administrationRefusedKey: InjectionKey<() => void> = Symbol('administration-refused')

export function useAdministrationRefusal() {
  const refused = inject(administrationRefusedKey, () => {})
  return (error: unknown) => {
    if (errorStatus(error) === 403) refused()
  }
}

export function formatDuration(seconds: number) {
  const days = Math.floor(seconds / 86_400)
  const hours = Math.floor((seconds % 86_400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days) return `${days}d ${hours}h`
  if (hours) return `${hours}h ${minutes}m`
  return `${minutes}m`
}
