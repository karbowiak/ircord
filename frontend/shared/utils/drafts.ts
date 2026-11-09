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

/** `limit` bounds the startup read; merging with another tab's save reads all of it. */
export function sanitizeDrafts(value: unknown, limit = DRAFT_LIMIT): Drafts {
  const drafts: Drafts = {}
  if (value && typeof value === 'object')
    for (const [id, text] of Object.entries(value).slice(0, limit))
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
  const line = text
    .slice(0, limit * 4)
    .replace(/\s+/g, ' ')
    .trim()
  const chars = Array.from(line)
  return chars.length > limit
    ? `${chars
        .slice(0, limit - 1)
        .join('')
        .trimEnd()}…`
    : line
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

/** Drafts with visible text: what the sidebar counts and the overview can list. */
export function draftCount(drafts: Drafts) {
  return Object.values(drafts).filter((text) => typeof text === 'string' && text.trim()).length
}

/**
 * Every non-empty draft as an overview row: newest edit first, then drafts from before edit
 * times were kept (by name), with drafts for conversations that are gone kept in the list.
 * Sorted before it is cut to `limit`, so the newest drafts are never the ones left out.
 * `workspace` is undefined while nothing is loaded yet: rows are then 'unknown', not gone.
 */
export function draftRows(
  drafts: Drafts,
  times: DraftTimes,
  workspace: Workspace | undefined,
  label: (c: Conversation) => string,
  limit = DRAFT_LIMIT,
  translate?: (key: string) => string,
  locale?: string,
): DraftRow[] {
  const networks = new Map(workspace?.networks.map((n) => [n.id, n]))
  const conversations = new Map(workspace?.conversations.map((c) => [c.id, c]))
  const rows: DraftRow[] = []
  for (const [id, text] of Object.entries(drafts)) {
    if (typeof text !== 'string' || !text.trim()) continue
    const base = { id, text, preview: '', editedAt: times[id] }
    if (id.startsWith('server:')) {
      const network = networks.get(id.slice('server:'.length))
      rows.push({
        ...base,
        kind: 'server',
        title: translate?.('workspaceUi.drafts.serverWindow') ?? 'Server window',
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
      title: c
        ? label(c)
        : workspace
          ? (translate?.('workspaceUi.drafts.unavailableConversation') ??
            'Unavailable conversation')
          : (translate?.('workspaceUi.drafts.conversation') ?? 'Conversation'),
      networkName: network?.name,
      state: !workspace ? 'unknown' : c && network ? 'available' : 'unavailable',
    })
  }
  return rows
    .sort(
      (a, b) =>
        (b.editedAt ?? 0) - (a.editedAt ?? 0) ||
        a.title.localeCompare(b.title, locale) ||
        (a.networkName ?? '').localeCompare(b.networkName ?? '', locale) ||
        a.id.localeCompare(b.id),
    )
    .slice(0, limit)
    .map((row) => ({ ...row, preview: draftPreview(row.text) }))
}

/** Removes a draft only if it still holds the text the caller saw, so newer text survives. */
export function discardDraft(drafts: Drafts, id: string, seen: string) {
  if (drafts[id] !== seen) return false
  delete drafts[id]
  return true
}

/** Puts discarded text back, but never over a draft written since. */
export function restoreDraft(
  drafts: Drafts,
  id: string,
  text: string,
): 'restored' | 'conflict' | 'full' {
  if (drafts[id]) return 'conflict'
  if (draftCount(drafts) >= DRAFT_LIMIT) return 'full'
  drafts[id] = text.slice(0, DRAFT_TEXT_LIMIT)
  return 'restored'
}

/**
 * Three-way merge of this tab's drafts (`mine`) with what another tab saved (`theirs`), both
 * descended from what this tab last saw saved (`base`). A draft this tab did not change takes
 * the saved value; one it did change keeps this tab's text, except that removing a draft never
 * wins over newer text another tab saved for it. Returns the merged drafts, the ids whose value
 * came from `theirs` (and differs from `mine`), and the new `base`: the saved value for drafts
 * taken from `theirs`, the old one for this tab's changes that are not saved yet.
 */
export function mergeDrafts(base: Drafts, mine: Drafts, theirs: Drafts) {
  const merged: Drafts = {}
  const adopted: string[] = []
  const seen: Drafts = { ...base }
  for (const id of new Set([...Object.keys(mine), ...Object.keys(theirs)])) {
    const changedHere = mine[id] !== base[id]
    const changedThere = theirs[id] !== base[id]
    const keepTheirs = !changedHere || (mine[id] === undefined && changedThere)
    const value = keepTheirs ? theirs[id] : mine[id]
    if (value) merged[id] = value
    if (keepTheirs && theirs[id] !== mine[id]) adopted.push(id)
    if (keepTheirs) {
      if (theirs[id]) seen[id] = theirs[id]
      else delete seen[id]
    }
  }
  return { merged, adopted, seen }
}
