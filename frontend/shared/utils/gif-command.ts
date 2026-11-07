import type { GifItem } from '../types/media'

export const gifUsage = 'Usage: /gif <search terms>'

/** Picks one item uniformly; `random` is injectable so tests are deterministic. */
export function pickRandom<T>(items: readonly T[], random: () => number = Math.random): T | undefined {
  if (!items.length) return undefined
  return items[Math.min(items.length - 1, Math.floor(random() * items.length))]
}

export interface GifCommandDeps {
  /** Matching results for a query (one provider page is plenty to pick from). */
  search: (query: string) => Promise<GifItem[]>
  /** Sends the GIF to a specific conversation; false when it can't be sent right now. */
  send: (conversationId: string, item: GifItem) => boolean
  /** Registers the share with the provider; must never throw or block. */
  share: (item: GifItem, query: string) => void
  notify: (text: string) => void
  random?: () => number
  /** Where in-flight lookups are tracked; pass a reactive Set to drive UI state. */
  busy?: Set<string>
}

/** `sent`: the GIF went out and the input can be cleared. `kept`: keep the input for a retry. */
export type GifCommandResult = 'sent' | 'kept'

/**
 * `/gif <query>`: search, pick one result at random and send it to the conversation the command
 * was submitted in, even if the user has navigated elsewhere by the time the search returns.
 * One lookup per conversation at a time, so repeated Enter presses can't send twice.
 */
export function createGifCommand(deps: GifCommandDeps) {
  const busy = deps.busy ?? new Set<string>()

  async function run(query: string, conversationId: string): Promise<GifCommandResult> {
    const q = query.trim()
    if (!q) {
      deps.notify(gifUsage)
      return 'kept'
    }
    if (busy.has(conversationId)) {
      deps.notify('Still looking for a GIF…')
      return 'kept'
    }
    busy.add(conversationId)
    try {
      const items = await deps.search(q)
      const item = pickRandom(items, deps.random)
      if (!item) {
        deps.notify(`No GIFs matched “${q}”. Try other words.`)
        return 'kept'
      }
      if (!deps.send(conversationId, item)) {
        deps.notify('Reconnect before sending this GIF.')
        return 'kept'
      }
      deps.share(item, q)
      return 'sent'
    } catch (error) {
      deps.notify(error instanceof Error && error.message ? error.message : 'Couldn’t find a GIF.')
      return 'kept'
    } finally {
      busy.delete(conversationId)
    }
  }

  return { run, isBusy: (conversationId: string) => busy.has(conversationId) }
}
