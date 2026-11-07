export default defineEventHandler(async (event) => {
  const session = await demoSession(event)
  const query = getQuery(event)
  const q = String(query.q ?? '')
    .trim()
    .toLowerCase()
    .slice(0, 200)
  if (q.length < 2) return { items: [] }
  const allowed = session.bootstrap.conversations.filter(
    (c) => !query.conversationId || c.id === query.conversationId,
  )
  const items = allowed
    .flatMap((conversation) =>
      (session.messages[conversation.id] ?? [])
        .filter((m) => m.text.toLowerCase().includes(q))
        .map((message) => ({ message, conversation })),
    )
    .sort((a, b) => b.message.createdAt.localeCompare(a.message.createdAt))
    .slice(0, 50)
  return { items }
})
