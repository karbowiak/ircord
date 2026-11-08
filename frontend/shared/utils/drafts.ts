import type { Conversation, Network } from '../types/chat'

/** Conversations whose composer text is kept, and the longest text kept for each. */
export const DRAFT_LIMIT = 100
export const DRAFT_TEXT_LIMIT = 4000
/** Characters of a draft shown in the Drafts overview. */
export const DRAFT_PREVIEW_LIMIT = 160

/** Composer text per conversation id: the stored payload, unchanged since drafts were added. */
export type Drafts = Record<string, string>
/** When each draft was last edited (ms since the epoch); legacy drafts have no entry. */
export type DraftTimes = Record<string, number>

export function sanitizeDrafts(value: unknown): Drafts {
  const drafts: Drafts = {}
  if (value && typeof value === 'object')
    for (const [id, text] of Object.entries(value).slice(0, DRAFT_LIMIT))
      if (typeof text === 'string' && text) drafts[id] = text.slice(0, DRAFT_TEXT_LIMIT)
  return drafts
}

/** Keeps only plausible times for drafts that still exist; anything else counts as unknown. */
export function sanitizeDraftTimes(value: unknown, drafts: Drafts): DraftTimes {
  const times: DraftTimes = {}
  if (value && typeof value === 'object')
    for (const [id, time] of Object.entries(value))
      if (id in drafts && typeof time === 'number' && Number.isFinite(time) && time > 0)
        times[id] = time
  return times
}

/**
 * Stamps drafts whose text changed since `previous` with `now` and forgets the times of drafts
 * that are gone. Drafts that did not change keep their time, or stay unknown (legacy drafts).
 * Returns whether any time changed.
 */
export function stampDraftTimes(previous: Drafts, current: Drafts, times: DraftTimes, now: number) {
  let changed = false
  for (const [id, text] of Object.entries(current))
    if (previous[id] !== text) {
      times[id] = now
      changed = true
    }
  for (const id of Object.keys(times))
    if (!(id in current)) {
      delete times[id]
      changed = true
    }
  return changed
}

/** A bounded single-line preview: whitespace collapsed, cut on a character boundary. */
export function draftPreview(text: string, limit = DRAFT_PREVIEW_LIMIT) {
  const line = text.slice(0, limit * 4).replace(/\s+/g, ' ').trim()
  const chars = Array.from(line)
  return chars.length > limit ? `${chars.slice(0, limit - 1).join('').trimEnd()}…` : line
}

export type DraftRow = {
  id: string
  text: string
  preview: string
  editedAt?: number
  /** 'unavailable': the conversation or its network is gone; the text is still kept. */
  state: 'available' | 'unavailable' | 'unknown'
  kind: 'channel' | 'direct' | 'server'
  title: string
  networkName?: string
}

type Workspace = { conversations: Conversation[]; networks: Network[] }

/**
 * Every non-empty draft as an overview row: newest edit first, then drafts from before edit
 * times were kept (by name), with drafts for conversations that are gone kept in the list.
 * `workspace` is undefined while nothing is loaded yet: rows are then 'unknown', not gone.
 */
export function draftRows(
  drafts: Drafts,
  times: DraftTimes,
  workspace: Workspace | undefined,
  label: (c: Conversation) => string,
): DraftRow[] {
  const networks = new Map(workspace?.networks.map((n) => [n.id, n]))
  const conversations = new Map(workspace?.conversations.map((c) => [c.id, c]))
  const rows: DraftRow[] = []
  for (const [id, text] of Object.entries(drafts).slice(0, DRAFT_LIMIT)) {
    if (typeof text !== 'string' || !text.trim()) continue
    const base = { id, text, preview: draftPreview(text), editedAt: times[id] }
    if (id.startsWith('server:')) {
      const network = networks.get(id.slice('server:'.length))
      rows.push({
        ...base,
        kind: 'server',
        title: 'Server window',
        networkName: network?.name,
        state: !workspace ? 'unknown' : network ? 'available' : 'unavailable',
      })
      continue
    }
    const c = conversations.get(id)
    const network = c && networks.get(c.networkId)
    rows.push({
      ...base,
      kind: c?.kind ?? 'channel',
      title: c ? label(c) : 'Unavailable conversation',
      networkName: network?.name,
      state: !workspace ? 'unknown' : c && network ? 'available' : 'unavailable',
    })
  }
  return rows.sort(
    (a, b) =>
      (b.editedAt ?? 0) - (a.editedAt ?? 0) ||
      a.title.localeCompare(b.title) ||
      (a.networkName ?? '').localeCompare(b.networkName ?? '') ||
      a.id.localeCompare(b.id),
  )
}

/** Removes a draft only if it still holds the text the caller saw, so newer text survives. */
export function discardDraft(drafts: Drafts, id: string, seen: string) {
  if (drafts[id] !== seen) return false
  delete drafts[id]
  return true
}

/** Puts discarded text back, but never over a draft written since. */
export function restoreDraft(drafts: Drafts, id: string, text: string) {
  if (drafts[id] || !text || Object.keys(drafts).length >= DRAFT_LIMIT) return false
  drafts[id] = text.slice(0, DRAFT_TEXT_LIMIT)
  return true
}
