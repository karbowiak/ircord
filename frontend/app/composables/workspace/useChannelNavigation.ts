/** Numbered destinations follow the sidebar: favorite channels first, then channels, each in
 * the sidebar's order (A to Z, or the order you dragged them into).
 * Unread filters and collapsed groups never renumber keyboard destinations. */
export function useChannelNavigation() {
  const chat = useChatStore()
  const order = useSidebarOrder()
  return computed(() => [
    ...order.items(chat.networkId, 'favorites').filter((c) => c.kind === 'channel' && !c.party),
    ...order.items(chat.networkId, 'channels'),
  ])
}
