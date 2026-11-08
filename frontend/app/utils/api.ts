import { normalizeTransport } from './transport'
import { createIRCordClient } from '../../shared/api/client'
import type { Appearance, StoredAppearance } from '../../shared/appearance'
import type { ClientPreferences, StoredPreferences } from '../../shared/preferences'
import type { PickerLibrary, SavedGif } from '../../shared/library'
import type { EmojiCatalog } from '../../shared/types/media'
let bases: { api: string; app: string } | undefined

export function configureApiBases(config: { apiBase: string; appApiBase: string }) {
  bases = { api: config.apiBase, app: config.appApiBase }
  client = undefined
}

// Read once inside the Nuxt context; later calls may come from timers and socket handlers.
function base() {
  if (!bases) {
    const config = useRuntimeConfig().public
    bases = { api: config.apiBase, app: config.appApiBase }
  }
  return bases
}

let client: ReturnType<typeof createIRCordClient> | undefined
/** The generated Go contract. Callers never assert their own response type. */
export function apiClient() {
  return client ??= createIRCordClient(base().api, {
    transform: normalizeTransport,
    onUnauthorized: () => window.dispatchEvent(new Event('ircord:session-expired')),
  })
}

/** Frontend-owned routes use their shared sanitizer types, with fixed operation shapes. */
export const appClient = {
  appearance: () => $fetch<StoredAppearance>(`${base().app}/appearance`, { credentials: 'include' }),
  saveAppearance: (settings: Appearance) => $fetch<StoredAppearance>(`${base().app}/appearance`, { credentials: 'include', method: 'PUT', body: { settings } }),
  preferences: () => $fetch<StoredPreferences>(`${base().app}/preferences`, { credentials: 'include' }),
  savePreferences: (preferences: ClientPreferences) => $fetch<StoredPreferences>(`${base().app}/preferences`, { credentials: 'include', method: 'PUT', body: { preferences } }),
  library: () => $fetch<PickerLibrary>(`${base().app}/library`, { credentials: 'include' }),
  updateLibrary: (body: { uses?: Record<string, number>; skinTone?: string }) => $fetch<PickerLibrary>(`${base().app}/library`, { credentials: 'include', method: 'PATCH', body }),
  saveGif: (body: SavedGif) => $fetch<PickerLibrary>(`${base().app}/library/gifs`, { credentials: 'include', method: 'POST', body }),
  removeGif: (id: string) => $fetch<PickerLibrary>(`${base().app}/library/gifs/${encodeURIComponent(id)}`, { credentials: 'include', method: 'DELETE' }),
  emojiCatalog: () => $fetch<EmojiCatalog>(`${base().app}/emoji/catalog`, { credentials: 'include' }),
}

export const errorText = (error: unknown) => {
  const data = (error as { data?: { detail?: string; title?: string; statusMessage?: string } })
    ?.data
  return (
    data?.detail ??
    data?.statusMessage ??
    data?.title ??
    'Couldn’t reach the server. Please try again.'
  )
}
