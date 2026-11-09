import {
  matchesSearch,
  type SearchFilters,
  type SearchHas,
} from '../../../shared/utils/search-filter'

export default defineEventHandler(async (event) => {
  const session = await demoSession(event)
  const query = getQuery(event)
  const invalid = (message: string): never => {
    throw createError({ statusCode: 422, message })
  }
  const single = (name: string) => {
    const value = query[name]
    if (Array.isArray(value)) return invalid(`Choose one ${name} value.`)
    return String(value ?? '')
  }
  const list = (name: string) => {
    const value = query[name]
    const values =
      value === undefined ? [] : Array.isArray(value) ? value.map(String) : [String(value)]
    if (values.length > 50) invalid('Choose at most 50 values for each search filter.')
    return values
  }
  const filters: SearchFilters = {
    from: list('from'),
    mentions: list('mentions'),
    in: list('conversationId'),
    has: list('has') as SearchHas[],
    author: (single('author') as SearchFilters['author']) || undefined,
  }
  if (filters.has.some((kind) => !['link', 'image', 'gif', 'video', 'file'].includes(kind)))
    invalid('has must be link, image, gif, video, or file.')
  if (filters.author && !['user', 'bot'].includes(filters.author))
    invalid('author must be user or bot.')
  const instant = (name: string) => {
    const raw = single(name)
    if (!raw) return undefined
    if (
      !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(raw) ||
      Number.isNaN(Date.parse(raw))
    )
      invalid(`${name} must be an RFC 3339 date and time.`)
    return Date.parse(raw)
  }
  const before = instant('before'),
    after = instant('after')
  if (before !== undefined && after !== undefined && before <= after)
    invalid('before must be later than after.')
  const q = single('q').trim()
  const filtered =
    filters.from.length + filters.mentions.length + filters.in.length + filters.has.length > 0 ||
    !!filters.author ||
    before !== undefined ||
    after !== undefined
  if ([...q].length > 200 || (!filtered && [...q].length < 2))
    invalid(
      'Enter at least two characters or choose a filter; search text can have at most 200 characters.',
    )
  const rawLimit = single('limit')
  const limit = rawLimit ? Number(rawLimit) : 50
  if (!Number.isInteger(limit) || limit < 1 || limit > 100)
    invalid('limit must be between 1 and 100.')
  const visible = session.bootstrap.conversations
  if (filters.in.some((id) => !visible.some((c) => c.id === id)))
    throw createError({ statusCode: 404, message: 'Conversation not found.' })
  const member = (id: string) =>
    session.bootstrap.members.find((m) => m.id === id) ??
    (session.bootstrap.user.id === id ? session.bootstrap.user : undefined)
  const from = filters.from.length,
    mentions = filters.mentions.length
  filters.from = filters.from.filter((id) => member(id))
  filters.mentions = filters.mentions.filter((id) => member(id))
  const empty = (from > 0 && !filters.from.length) || (mentions > 0 && !filters.mentions.length)
  let cursor: { at: string; id: string } | undefined
  const rawCursor = single('cursor')
  if (rawCursor) {
    try {
      if (rawCursor.length > 512) throw new Error()
      const value = JSON.parse(Buffer.from(rawCursor, 'base64url').toString())
      if (
        typeof value.at !== 'string' ||
        Number.isNaN(Date.parse(value.at)) ||
        typeof value.id !== 'string' ||
        !value.id
      )
        throw new Error()
      cursor = value
    } catch {
      throw createError({ statusCode: 400, message: 'Invalid cursor.' })
    }
  }
  const items = (empty ? [] : visible)
    .flatMap((conversation) =>
      (session.messages[conversation.id] ?? [])
        .filter((m) => {
          const at = Date.parse(m.createdAt)
          return (
            !m.deleted &&
            matchesSearch(m, q, filters, member) &&
            (before === undefined || at < before) &&
            (after === undefined || at >= after) &&
            (!cursor || m.createdAt < cursor.at || (m.createdAt === cursor.at && m.id < cursor.id))
          )
        })
        .map((message) => ({ message, conversation })),
    )
    .sort(
      (a, b) =>
        b.message.createdAt.localeCompare(a.message.createdAt) ||
        (a.message.id < b.message.id ? 1 : a.message.id > b.message.id ? -1 : 0),
    )
  const page = items.slice(0, limit)
  const last = page.at(-1)?.message
  const nextCursor =
    items.length > limit && last
      ? Buffer.from(JSON.stringify({ at: last.createdAt, id: last.id })).toString('base64url')
      : null
  return { items: page, nextCursor }
})
