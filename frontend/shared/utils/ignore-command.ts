// `/ignore` and `/unignore`: edit the same ignore list the user menu and settings use.

export interface IgnoreOutcome {
  /** The new ignore list, or undefined when nothing changes. */
  ignore?: string[]
  notice: string
}

const same = (a: string, b: string) => a.toLowerCase() === b.toLowerCase()

/**
 * `/ignore` lists who is ignored, `/ignore <nick|mask>` adds an entry and `/unignore <nick|mask>`
 * removes one. Entries are nicks or IRC masks (`nick!user@host`), compared case-insensitively;
 * a leading “@” from mention completion is dropped.
 */
export function ignoreCommand(
  name: 'ignore' | 'unignore',
  arg: string,
  list: readonly string[],
): IgnoreOutcome {
  const target = arg.trim().replace(/^@/, '')
  if (!target) {
    if (name === 'unignore') return { notice: 'Usage: /unignore <nick>' }
    return {
      notice: list.length
        ? `Ignoring ${list.length === 1 ? '1 person' : `${list.length} people`}: ${list.join(', ')}`
        : 'You aren’t ignoring anyone. Use /ignore <nick> to start.',
    }
  }
  if (/\s/.test(target)) return { notice: `Usage: /${name} <nick>` }
  const exists = list.some((mask) => same(mask, target))
  if (name === 'ignore') {
    if (exists) return { notice: `You’re already ignoring ${target}.` }
    return {
      ignore: [...list, target],
      notice: `Ignoring ${target}. Their messages are hidden; /unignore ${target} to undo.`,
    }
  }
  if (!exists) return { notice: `You aren’t ignoring ${target}.` }
  return {
    ignore: list.filter((mask) => !same(mask, target)),
    notice: `No longer ignoring ${target}.`,
  }
}
