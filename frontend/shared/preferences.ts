// Display preferences that follow the account: event display,
// network icons, embed loading, keyboard shortcuts and conversation-list order.
// Shared by the Nitro route (validation) and the browser. Notification rules belong to the
// backend (shared/notifications.ts); device-specific choices stay in the browser.

import { emptySpeechService, sanitizeSpeechService, type SpeechService } from './speech-service'
import { sanitizeShortcuts, type ShortcutOverrides } from './utils/shortcuts'
import { sanitizeSidebarOrder, type SidebarOrder } from './utils/sidebar-order'

export interface ClientPreferences {
  /** Startup choice follows the account; the last viewed conversation stays on this device. */
  startup: { mode: 'last' | 'channel'; channelId: string | null }
  /** Join, part, quit and nick changes: shown, collapsed into one line, or hidden. */
  events: 'show' | 'collapse' | 'hide'
  /** Custom network icons as small image data URLs. */
  networkIcons: Record<string, string>
  /**
   * Channels you create on the IRCord network are registered to your account with ChanServ.
   * The backend reads this from the stored document.
   */
  registerChannels: boolean
  /** Require a click before loading third-party players and widgets. */
  askBeforeEmbeds: boolean
  /** Private per-server opt-ins for NSFW links. */
  nsfwNetworks: Record<string, boolean>
  /**
   * Read-aloud's speech service: address, model and voice only. Its API key is kept per device
   * and never stored here. The page's Content-Security-Policy allows this service's origin.
   */
  speech: SpeechService
  /** Keyboard shortcuts you changed; every other action keeps its default keys. */
  shortcuts: ShortcutOverrides
  /** Conversation-list sections you put in your own order (see utils/sidebar-order.ts). */
  sidebarOrder: SidebarOrder
}
export interface StoredPreferences {
  preferences: ClientPreferences | null
  updatedAt: string | null
}

export const defaultPreferences: ClientPreferences = {
  startup: { mode: 'channel', channelId: null },
  events: 'collapse',
  networkIcons: {},
  registerChannels: true,
  askBeforeEmbeds: false,
  nsfwNetworks: {},
  speech: emptySpeechService,
  shortcuts: {},
  sidebarOrder: {},
}
const validId = (id: string) => id.length > 0 && id.length <= 120
export const maxIconBytes = 64 * 1024

export function sanitizePreferences(input: unknown): ClientPreferences {
  const v = (input && typeof input === 'object' ? input : {}) as Record<string, unknown>
  const startup = (v.startup && typeof v.startup === 'object' ? v.startup : {}) as Record<
    string,
    unknown
  >
  const record = (value: unknown) =>
    value && typeof value === 'object' ? Object.entries(value) : []
  return {
    startup: {
      mode: startup.mode === 'last' ? 'last' : 'channel',
      channelId:
        typeof startup.channelId === 'string' && validId(startup.channelId)
          ? startup.channelId
          : null,
    },
    events: v.events === 'show' || v.events === 'hide' ? v.events : 'collapse',
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
    registerChannels: v.registerChannels !== false,
    askBeforeEmbeds: v.askBeforeEmbeds === true,
    nsfwNetworks: Object.fromEntries(
      record(v.nsfwNetworks)
        .filter(
          ([id, enabled]) => /^[A-Za-z0-9][A-Za-z0-9._:-]{0,119}$/.test(id) && enabled === true,
        )
        .slice(0, 100),
    ),
    speech: sanitizeSpeechService(v.speech),
    shortcuts: sanitizeShortcuts(v.shortcuts),
    sidebarOrder: sanitizeSidebarOrder(v.sidebarOrder),
  }
}
