/** A build as `/_nuxt/builds/latest.json` describes it. */
export interface BuildInfo {
  id: string
  timestamp?: number
}

/**
 * True when `latest` (the parsed `/_nuxt/builds/latest.json`) names a build this page should move
 * to: a different id, and not an older one when both build times are known. During a rolling
 * update a tab already on the new build can reach a replica still serving the old one; with the
 * page's own build time on record that is not reported as an update.
 */
export function isNewerBuild(current: BuildInfo, latest: unknown): latest is BuildInfo {
  if (!current.id || !latest || typeof latest !== 'object') return false
  const { id, timestamp } = latest as { id?: unknown; timestamp?: unknown }
  if (typeof id !== 'string' || id === '' || id === current.id) return false
  return !(
    typeof timestamp === 'number' &&
    typeof current.timestamp === 'number' &&
    timestamp < current.timestamp
  )
}

/** What a reload would lose, read at the moment of deciding. */
export interface ReloadState {
  hidden: boolean
  /** The tab has stayed hidden long enough that the user has moved on, not just glanced away. */
  hiddenLongEnough: boolean
  outboxEmpty: boolean
  uploadsPending: boolean
  /** A dialog, menu or picker is open: its form state lives only in memory. */
  overlayOpen: boolean
  /** A text field other than the (saved) composer holds typed text. */
  unsavedInput: boolean
}

/**
 * Whether the page may reload to a new build without asking. Only a tab nobody has looked at for a
 * while, with nothing that lives only in memory, qualifies; everything else waits for a click.
 */
export function canReloadSilently(state: ReloadState): boolean {
  return (
    state.hidden &&
    state.hiddenLongEnough &&
    state.outboxEmpty &&
    !state.uploadsPending &&
    !state.overlayOpen &&
    !state.unsavedInput
  )
}
