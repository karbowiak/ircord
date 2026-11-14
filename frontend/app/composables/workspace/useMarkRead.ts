/** Marking several conversations read at once (a network, or everything). */
export function useMarkRead() {
  const chat = useChatStore()
  const unread = (networkId?: string) =>
    (chat.data?.conversations ?? []).filter(
      (c) =>
        (!networkId || c.networkId === networkId) &&
        (c.unreadCount || c.mentionCount || c.unreadFrom),
    )
  async function mark(networkId?: string) {
    const list = unread(networkId)
    await Promise.all(list.map((c) => chat.markRead(c.id)))
    return list.length
  }
  return {
    unread,
    network: (networkId: string) => mark(networkId),
    all: () => mark(),
  }
}
