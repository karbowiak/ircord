const key = 'ircord.recent'
const recent = ref<string[]>(
  readLocal(key, (v) =>
    Array.isArray(v) ? v.filter((x) => typeof x === 'string').slice(0, 30) : [],
  ),
)

/** Conversations in the order they were last opened on this device (quick switcher ranking). */
export function useRecentConversations() {
  function visit(id: string) {
    if (!id) return
    recent.value = [id, ...recent.value.filter((x) => x !== id)].slice(0, 30)
    writeLocal(key, recent.value)
  }
  return { recent, visit }
}
