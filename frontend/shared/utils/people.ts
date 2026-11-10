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

export function personByNick<T extends NamedPerson>(
  people: readonly T[],
  nick: string,
  mapping = '',
) {
  const target = foldPersonIdentity(nick, mapping)
  return people.find((person) => foldPersonIdentity(person.nick, mapping) === target)
}
