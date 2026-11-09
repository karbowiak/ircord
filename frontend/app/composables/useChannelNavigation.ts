/** Numbered destinations follow the sidebar: favorites first, then channel names.
 * Unread filters and collapsed groups never renumber keyboard destinations. */
export function useChannelNavigation() {
  const chat = useChatStore()
  return computed(() =>
    (chat.data?.conversations ?? [])
      .filter((c) => c.networkId === chat.networkId && c.kind === 'channel')
      .sort(
        (a, b) =>
          Number(!!b.favorite) - Number(!!a.favorite) ||
          a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }),
      ),
  )
}
