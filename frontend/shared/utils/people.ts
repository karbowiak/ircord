/** IRC identity comparison follows the selected network's advertised casemapping. */
export function foldPersonIdentity(value: string, mapping = '') {
  const mode = mapping.toLowerCase()
  return [...value]
    .map((character) => {
      if (mode && !['ascii', 'rfc1459', 'strict-rfc1459'].includes(mode))
        return [...character.toLowerCase()][0]!
      if (character >= 'A' && character <= 'Z') return character.toLowerCase()
      if (mode === 'ascii') return character
      if (character === '[') return '{'
      if (character === ']') return '}'
      if (character === '\\') return '|'
      if (character === '~' && mode !== 'strict-rfc1459') return '^'
      return character
    })
    .join('')
}

interface NamedPerson {
  id: string
  nick: string
  displayName?: string
  status?: string
}
interface ScopedConversation {
  networkId: string
  memberIds: readonly string[]
}

/** Global history filters can include every visible network. */
export function otherPeople<T extends NamedPerson>(members: readonly T[]) {
  return members.filter((member) => member.id !== 'you')
}

/** Local members proven to belong to a network by a retained conversation or scoped ID. */
export function networkPeople<T extends NamedPerson>(
  members: readonly T[],
  conversations: readonly ScopedConversation[],
  networkId: string,
) {
  if (!networkId) return []
  const ids = new Set(
    conversations.filter((c) => c.networkId === networkId).flatMap((c) => c.memberIds),
  )
  return otherPeople(members).filter((m) => ids.has(m.id) || m.id.startsWith(`${networkId}:`))
}

/** Rank locally available nick targets without treating a services account as a nickname. */
export function matchingPeople<T extends NamedPerson>(
  people: readonly T[],
  query: string,
  mapping = '',
  limit = 50,
) {
  const q = foldPersonIdentity(query.trim(), mapping)
  const text = query.trim().toLowerCase()
  const status: Record<string, number> = { online: 0, away: 1, offline: 2 }
  return people
    .map((person) => {
      const nick = foldPersonIdentity(person.nick, mapping)
      const name = (person.displayName || person.nick).toLowerCase()
      const rank = name.startsWith(text)
        ? 0
        : nick.startsWith(q)
          ? 1
          : nick.includes(q) || name.includes(text)
            ? 2
            : Infinity
      return { person, rank }
    })
    .filter((item) => item.rank < Infinity)
    .sort(
      (a, b) =>
        a.rank - b.rank ||
        (status[a.person.status ?? ''] ?? 3) - (status[b.person.status ?? ''] ?? 3) ||
        (a.person.displayName || a.person.nick).localeCompare(
          b.person.displayName || b.person.nick,
        ),
    )
    .slice(0, Math.max(0, Math.min(limit, 50)))
    .map((item) => item.person)
}

/**
 * A direct conversation's other person. Its member list names them when known; a conversation
 * that lists only you (opened by nick) is matched by its name instead.
 */
export function directPartnerOf<T>(
  conversation: { kind: string; name: string; memberIds: readonly string[]; party?: unknown },
  byId: (id: string) => T | undefined,
  byNick: (nick: string) => T | undefined,
) {
  const id = conversation.memberIds.find((memberId) => memberId !== 'you')
  if (id) return byId(id)
  return conversation.kind === 'direct' && !conversation.party
    ? byNick(conversation.name)
    : undefined
}

/**
 * Nick lookups over a large member list. Folding every nick is the costly part, and people keep
 * the same object across most member list changes, so each one's folded nick is remembered.
 */
export function createNickIndex<T extends NamedPerson>() {
  const folded = new WeakMap<T, Map<string, string>>()
  /** A person's nick folded with a network's casemapping. */
  function fold(person: T, mapping = '') {
    let byMapping = folded.get(person)
    if (!byMapping) folded.set(person, (byMapping = new Map()))
    let value = byMapping.get(mapping)
    if (value === undefined) {
      value = foldPersonIdentity(person.nick, mapping)
      byMapping.set(mapping, value)
    }
    return value
  }
  /** People by folded nick for each casemapping, each list in the order given. */
  function index(people: readonly T[], mappings: readonly string[]) {
    const result = new Map<string, Map<string, T[]>>()
    for (const mapping of new Set(mappings)) {
      const byNick = new Map<string, T[]>()
      for (const person of people) {
        const key = fold(person, mapping)
        const list = byNick.get(key)
        if (list) list.push(person)
        else byNick.set(key, [person])
      }
      result.set(mapping, byNick)
    }
    return result
  }
  return { fold, index }
}

export function personByNick<T extends NamedPerson>(
  people: readonly T[],
  nick: string,
  mapping = '',
) {
  const target = foldPersonIdentity(nick, mapping)
  return people.find((person) => foldPersonIdentity(person.nick, mapping) === target)
}
