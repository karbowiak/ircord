/** A superseded navigation is expected; actual history failures still reach the normal notice. */
export async function settleHistoryRefresh(
  refresh: () => Promise<{ error: unknown }>,
  report: (error: unknown) => void,
): Promise<boolean> {
  try {
    const state = await refresh()
    if (!state.error) return true
    report(state.error)
  } catch (error) {
    if (!(error && typeof error === 'object' && 'name' in error && error.name === 'AbortError'))
      report(error)
  }
  return false
}
