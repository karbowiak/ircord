// Display preferences that follow the account: event display, link previews and network icons.
// Shared by the Nitro route (validation) and the browser. Notification rules belong to the
// backend (shared/notifications.ts); device-specific choices stay in the browser.

export interface ClientPreferences {
  /** Join, part, quit and nick changes: shown, collapsed into one line, or hidden. */
  events: 'show' | 'collapse' | 'hide'
  /**
   * Inline previews of linked images and videos. Loading them tells the linked site your IP
   * address, so the default only previews well-known image hosts.
   */
  linkPreviews: 'off' | 'trusted' | 'all'
  /** Custom network icons as small image data URLs. */
  networkIcons: Record<string, string>
}
export interface StoredPreferences {
  preferences: ClientPreferences | null
  updatedAt: string | null
}

export const defaultPreferences: ClientPreferences = {
  events: 'collapse',
  linkPreviews: 'trusted',
  networkIcons: {},
}
const validId = (id: string) => id.length > 0 && id.length <= 120
export const maxIconBytes = 64 * 1024

export function sanitizePreferences(input: unknown): ClientPreferences {
  const v = (input && typeof input === 'object' ? input : {}) as Record<string, unknown>
  const record = (value: unknown) =>
    value && typeof value === 'object' ? Object.entries(value) : []
  return {
    events: v.events === 'show' || v.events === 'hide' ? v.events : 'collapse',
    linkPreviews: v.linkPreviews === 'off' || v.linkPreviews === 'all' ? v.linkPreviews : 'trusted',
    networkIcons: Object.fromEntries(
      record(v.networkIcons)
        .filter(
          ([id, url]) =>
            validId(id) &&
            typeof url === 'string' &&
            url.length <= maxIconBytes &&
            /^data:image\/(png|jpeg|webp);base64,[\w+/=]+$/.test(url),
        )
        .slice(0, 30),
    ),
  }
}
