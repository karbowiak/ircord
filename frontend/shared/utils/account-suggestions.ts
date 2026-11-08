// Account suggestions for ChanServ forms. ChanServ access and ownership follow services accounts,
// so a person in the channel is only offered under the account the network reports for them; a
// nick is never taken as an account.

/** The parts of a channel member the suggestions read. */
export interface RosterPerson {
  id: string
  nick: string
  displayName?: string
  account?: string
}

export interface AccountSuggestion {
  account: string
  /** Everyone in the channel logged in to it, the best match first (several sessions or nicks). */
  nicks: string[]
  /** A display name that differs from the nick, for context. */
  name?: string
}

/** A matching person without a known account: shown, explained, never selectable. */
export interface UnknownPerson {
  nick: string
  name?: string
}

const fold = (value: string) => value.toLowerCase()

/** How well `query` matches; lower is better, Infinity is no match. */
function rank(query: string, person: RosterPerson) {
  if (!query) return 5
  const account = fold(person.account ?? '')
  const nick = fold(person.nick)
  const name = fold(person.displayName ?? '')
  if (account === query || nick === query) return 0
  if (account.startsWith(query)) return 1
  if (nick.startsWith(query)) return 2
  if (name.startsWith(query)) return 3
  if (account.includes(query) || nick.includes(query) || name.includes(query)) return 4
  return Infinity
}

/**
 * Suggestions for `query` from the channel's people: one entry per account (several nicks on one
 * account are merged), and matching people whose account is unknown, separately. `exclude` drops
 * accounts that make no sense as a target, such as your own.
 */
export function accountSuggestions(
  people: readonly RosterPerson[],
  query: string,
  { exclude = [], limit = 8 }: { exclude?: readonly string[]; limit?: number } = {},
) {
  const q = fold(query.trim())
  const skip = new Set(exclude.filter(Boolean).map(fold))
  const byAccount = new Map<string, { best: number; people: { rank: number; p: RosterPerson }[] }>()
  const unknown: { rank: number; p: RosterPerson }[] = []
  for (const p of people) {
    const r = rank(q, p)
    const account = p.account?.trim()
    if (!account || account === '*') {
      // Only explain unknown accounts when someone is looked for by name.
      if (q && r < Infinity) unknown.push({ rank: r, p })
      continue
    }
    const key = fold(account)
    if (skip.has(key)) continue
    const entry = byAccount.get(key) ?? { best: Infinity, people: [] }
    entry.people.push({ rank: r, p })
    entry.best = Math.min(entry.best, r)
    byAccount.set(key, entry)
  }
  const order = (a: { rank: number; p: RosterPerson }, b: { rank: number; p: RosterPerson }) =>
    a.rank - b.rank || a.p.nick.localeCompare(b.p.nick)
  const accounts: AccountSuggestion[] = [...byAccount.values()]
    .filter((e) => e.best < Infinity)
    .map((e) => {
      const members = e.people.sort(order)
      const first = members[0]!.p
      return {
        best: e.best,
        account: first.account!.trim(),
        nicks: members.map((m) => m.p.nick),
        name:
          first.displayName && fold(first.displayName) !== fold(first.nick)
            ? first.displayName
            : undefined,
      }
    })
    .sort((a, b) => a.best - b.best || a.account.localeCompare(b.account))
    .slice(0, limit)
    .map(({ best: _, ...s }) => s)
  return {
    accounts,
    unknown: unknown
      .sort(order)
      .slice(0, 3)
      .map(({ p }) => ({
        nick: p.nick,
        name: p.displayName && fold(p.displayName) !== fold(p.nick) ? p.displayName : undefined,
      })),
  }
}

/**
 * The channel member using `text` as a nick while logged in to a different account, so typing
 * a nick can be pointed at the account it actually belongs to.
 */
export function nickWithOtherAccount(people: readonly RosterPerson[], text: string) {
  const t = fold(text.trim())
  if (!t) return undefined
  const p = people.find((m) => fold(m.nick) === t)
  if (!p) return undefined
  const account = p.account?.trim()
  if (account && account !== '*' && fold(account) === t) return undefined
  // Someone else's account with this exact name still matches the text; don't second-guess it.
  if (people.some((m) => m.account && fold(m.account) === t)) return undefined
  return { nick: p.nick, account: account && account !== '*' ? account : undefined }
}
