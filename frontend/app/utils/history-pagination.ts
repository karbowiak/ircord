import type { MessagePage } from '../../shared/types/chat'
import { appendMessagePage, prependMessagePage } from '../../shared/utils/message-page'

export type HistoryDirection = 'older' | 'newer'
type HistoryQuery = { before?: string; around?: string; limit: number }

/** One bounded history read at a time; navigation releases its slot even before HTTP settles. */
export function createHistoryPagination(options: {
  pageSize: number
  limit: number
  get: (id: string) => MessagePage | undefined
  set: (id: string, page: MessagePage) => void
  read: (id: string, query: HistoryQuery, signal: AbortSignal) => Promise<MessagePage>
  changed: (direction?: HistoryDirection) => void
  report: (error: unknown) => void
}) {
  let active: { id: string; controller: AbortController } | undefined
  function cancel() {
    const previous = active
    active = undefined
    previous?.controller.abort()
    options.changed()
  }
  async function load(id: string, direction: HistoryDirection): Promise<boolean | undefined> {
    const page = options.get(id)
    const boundary = page?.items.at(-1)?.id
    if (
      active ||
      !page ||
      page.partial ||
      (direction === 'older' ? !page.nextCursor : !page.hasNewer || !boundary)
    )
      return
    const ticket = { id, controller: new AbortController() }
    active = ticket
    options.changed(direction)
    try {
      const result = await options.read(
        id,
        direction === 'older'
          ? { before: page.nextCursor!, limit: options.pageSize }
          : { around: boundary, limit: Math.min(100, options.pageSize * 2 + 1) },
        ticket.controller.signal,
      )
      if (ticket.controller.signal.aborted || active !== ticket) return
      const current = options.get(id)
      // Live revisions may change this page; a replacement/trim must keep its own boundaries.
      if (
        !current ||
        current.nextCursor !== page.nextCursor ||
        current.items.at(-1)?.id !== boundary
      )
        return
      const merged =
        direction === 'older'
          ? prependMessagePage(current, result, options.limit)
          : appendMessagePage(current, result, options.limit)
      if (
        direction === 'older' &&
        !result.syncError &&
        merged.nextCursor === current.nextCursor &&
        merged.items[0]?.id === current.items[0]?.id
      )
        throw new Error('History page did not advance')
      options.set(id, merged)
      return true
    } catch (error) {
      if (!ticket.controller.signal.aborted && active === ticket) {
        options.report(error)
        return false
      }
    } finally {
      if (active === ticket) {
        active = undefined
        options.changed()
      }
    }
  }
  return { load, cancel }
}
