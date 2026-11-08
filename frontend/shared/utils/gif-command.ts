import type { Message } from '../types/chat'
import type { GifItem } from '../types/media'

export const gifUsage = 'Usage: /gif <search terms>'

/** Picks one item uniformly; `random` is injectable so tests are deterministic. */
export function pickRandom<T>(items: readonly T[], random: () => number = Math.random): T | undefined {
  if (!items.length) return undefined
  return items[Math.min(items.length - 1, Math.floor(random() * items.length))]
}

/** Where the GIF goes, fixed when the command is submitted. */
export interface GifTarget {
  conversationId: string
  replyTo?: Message
}

export interface GifCommandDeps {
  /** Matching results for a query (one provider page is plenty to pick from). */
  search: (query: string) => Promise<GifItem[]>
  /** Sends the GIF to the target; false when it can't be sent right now. */
  send: (target: GifTarget, item: GifItem) => boolean
  /** Registers the share with the provider; must never throw or block. */
  share: (item: GifItem, query: string) => void
  notify: (text: string) => void
  random?: () => number
  /**
   * In-flight lookups, conversation id → query. Pass a reactive Map to drive UI state; it holds
   * an entry exactly as long as the lookup runs.
   */
  lookups?: Map<string, string>
}

/** `sent`: the GIF went out and the input can be cleared. `kept`: keep the input for a retry. */
export type GifCommandResult = 'sent' | 'kept'

/**
 * `/gif <query>`: search, pick one result at random and send it to the conversation the command
 * was submitted in, even if the user has navigated elsewhere by the time the search returns.
 * One lookup per conversation at a time, so repeated Enter presses can't send twice.
 */
export function createGifCommand(deps: GifCommandDeps) {
  const lookups = deps.lookups ?? new Map<string, string>()

  async function run(query: string, target: GifTarget): Promise<GifCommandResult> {
    const q = query.trim()
    if (!q) {
      deps.notify(gifUsage)
      return 'kept'
    }
    if (lookups.has(target.conversationId)) {
      deps.notify('Still looking for a GIF…')
      return 'kept'
    }
    lookups.set(target.conversationId, q)
    try {
      const items = await deps.search(q)
      const item = pickRandom(items, deps.random)
      if (!item) {
        deps.notify(`No GIFs matched “${q}”. Try other words.`)
        return 'kept'
      }
      if (!deps.send(target, item)) {
        deps.notify('Reconnect before sending this GIF.')
        return 'kept'
      }
      deps.share(item, q)
      return 'sent'
    } catch (error) {
      deps.notify(error instanceof Error && error.message ? error.message : 'Couldn’t find a GIF.')
      return 'kept'
    } finally {
      lookups.delete(target.conversationId)
    }
  }

  return { run, lookup: (conversationId: string) => lookups.get(conversationId) }
}
