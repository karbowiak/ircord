// Builds list-mode masks (bans, exceptions, invite exceptions) from a person instead of typed
// IRC syntax. The mask stays the single source of truth: which parts are "checked" is read back
// from it, so editing the mask by hand and ticking boxes never disagree.

import { foldPersonIdentity } from './people'

/** The parts of a person a mask can be built from. */
export interface MaskPerson {
  id: string
  nick: string
  displayName?: string
  username?: string
  host?: string
  account?: string
}

export type MaskPart = 'nick' | 'user' | 'host' | 'account'
export type MaskParts = Record<MaskPart, boolean>

const fold = (value: string) => value.toLowerCase()
/** A services account the network reported; `*` and empty mean not logged in. */
export const knownAccount = (p: MaskPerson) => {
  const a = p.account?.trim()
  return a && a !== '*' ? a : undefined
}

/** Splits `nick!user@host` the way `expandMask` writes it; anything else gives undefined. */
function splitMask(mask: string) {
  const bang = mask.indexOf('!')
  const at = mask.lastIndexOf('@')
  if (bang <= 0 || at < bang) return undefined
  return { nick: mask.slice(0, bang), user: mask.slice(bang + 1, at), host: mask.slice(at + 1) }
}

/**
 * Which parts of `person` the mask pins down exactly. A part counts as checked only when it's
 * the person's own value, so a hand-edited wildcard (`*.example.net`) leaves its box unticked.
 */
export function partsOf(mask: string, person: MaskPerson, accountPrefix?: string): MaskParts {
  const none: MaskParts = { nick: false, user: false, host: false, account: false }
  const m = mask.trim()
  const account = knownAccount(person)
  if (accountPrefix && account && fold(m) === fold(`${accountPrefix}${account}`))
    return { ...none, account: true }
  const parts = splitMask(m)
  if (!parts) return none
  const same = (part: string, value?: string) => !!value && fold(part) === fold(value)
  return {
    nick: same(parts.nick, person.nick),
    user: same(parts.user, person.username),
    host: same(parts.host, person.host),
    account: false,
  }
}

/**
 * The mask for `person` with only the checked parts pinned and the rest `*`. An account mask
 * stands alone: servers can't combine it with nick, user or host in one entry.
 */
export function buildMask(person: MaskPerson, parts: MaskParts, accountPrefix?: string) {
  const account = knownAccount(person)
  if (parts.account && accountPrefix && account) return `${accountPrefix}${account}`
  const nick = parts.nick ? person.nick : '*'
  const user = parts.user && person.username ? person.username : '*'
  const host = parts.host && person.host ? person.host : '*'
  return `${nick}!${user}@${host}`
}

/**
 * Turns one part on or off for `person`, starting from the parts the current mask pins. Turning
 * the account on replaces the address parts; turning an address part on drops the account.
 */
export function toggledMask(
  mask: string,
  person: MaskPerson,
  part: MaskPart,
  on: boolean,
  accountPrefix?: string,
) {
  const current = partsOf(mask, person, accountPrefix)
  const next: MaskParts =
    part === 'account'
      ? { nick: false, user: false, host: false, account: on }
      : { ...current, account: false, [part]: on }
  return buildMask(person, next, accountPrefix)
}

/**
 * The mask a newly chosen person starts with: their account where the network has account masks
 * and reports one (it follows them across nicks and hosts), else their host, else their nick.
 */
export function defaultMask(person: MaskPerson, accountPrefix?: string) {
  if (accountPrefix && knownAccount(person))
    return buildMask(
      person,
      { nick: false, user: false, host: false, account: true },
      accountPrefix,
    )
  if (person.host)
    return buildMask(person, { nick: false, user: false, host: true, account: false })
  return buildMask(person, { nick: true, user: false, host: false, account: false })
}

/** How well `query` matches a person; lower is better, Infinity is no match. */
function rank(query: string, p: MaskPerson, fold: (value: string) => string) {
  if (!query) return 5
  const nick = fold(p.nick)
  const name = fold(p.displayName ?? '')
  const account = fold(knownAccount(p) ?? '')
  if (nick === query) return 0
  if (nick.startsWith(query)) return 1
  if (account === query || account.startsWith(query)) return 2
  if (name.startsWith(query)) return 3
  if (nick.includes(query) || name.includes(query) || account.includes(query)) return 4
  return Infinity
}

/**
 * People to offer for `query`: those in the channel first, then people seen there recently
 * (`recent`, newest first) who aren't in it any more. Nicks are unique on a network, so a
 * recent person sharing a nick with someone present is dropped.
 */
export function personSuggestions(
  here: readonly MaskPerson[],
  recent: readonly MaskPerson[],
  query: string,
  limit = 8,
  caseMapping?: string,
) {
  const fold = (value: string) => foldPersonIdentity(value, caseMapping)
  const q = fold(query.trim())
  const seen = new Set(here.map((p) => fold(p.nick)))
  const away = recent.filter((p) => {
    const key = fold(p.nick)
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
  const order = (list: readonly MaskPerson[], keepOrder: boolean) =>
    list
      .map((p, i) => ({ p, r: rank(q, p, fold), i }))
      .filter((x) => x.r < Infinity)
      .sort((a, b) => a.r - b.r || (keepOrder ? a.i - b.i : a.p.nick.localeCompare(b.p.nick)))
      .map((x) => x.p)
  const people = [
    ...order(here, false).map((p) => ({ person: p, recent: false })),
    ...order(away, true).map((p) => ({ person: p, recent: true })),
  ]
  return people.slice(0, limit)
}
