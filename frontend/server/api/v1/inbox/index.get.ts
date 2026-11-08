import type { InboxFilter } from '../../../../shared/types/chat'
import { listInbox } from '../../../utils/demo-inbox'
const filters: InboxFilter[] = ['all', 'unread', 'mentions', 'replies', 'direct', 'dismissed']
export default defineEventHandler(async (event) => {
  try {
    const q = getQuery(event)
    const filter = typeof q.filter === 'string' ? q.filter : 'all'
    if (!filters.includes(filter as InboxFilter))
      throw createError({ statusCode: 422, statusMessage: 'Unknown inbox filter.' })
    return listInbox(
      await demoSession(event),
      filter as InboxFilter,
      typeof q.cursor === 'string' ? q.cursor : '',
      Number(q.limit) || 30,
    )
  } catch (error) {
    if ((error as { statusCode?: number }).statusCode) throw error
    routeError(error)
  }
})
