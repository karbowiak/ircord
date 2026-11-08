// `/ignore` and `/unignore`: edit the same ignore list the user menu and settings use.
import { maskMatches, notificationLimits } from '../notifications'

export interface IgnoreOutcome {
  /** The new ignore list, or undefined when nothing changes. */
  ignore?: string[]
  notice: string
}

/** Who a nick or mask is checked against: its IRC identity when known. */
export interface IgnoreIdentity {
  nick: string
  user?: string
  host?: string
}

const same = (a: string, b: string) => a.toLowerCase() === b.toLowerCase()
/** Whether the list has this exact entry (case-insensitive). */
export const hasIgnore = (list: readonly string[], entry: string) =>
  list.some((mask) => same(mask, entry))
export const addIgnore = (list: readonly string[], entry: string) =>
  hasIgnore(list, entry) ? [...list] : [...list, entry]
export const removeIgnore = (list: readonly string[], entry: string) =>
  list.filter((mask) => !same(mask, entry))
const covering = (list: readonly string[], who: IgnoreIdentity) =>
  list.filter((mask) => maskMatches(mask, who.nick, who.user, who.host))

/**
 * `/ignore` lists who is ignored, `/ignore <nick|mask>` adds an entry and `/unignore <nick|mask>`
 * removes one. Entries are nicks or IRC masks (`nick!user@host`), compared case-insensitively;
 * a leading “@” is dropped. `person` is the target's identity when they are known, so masks
 * that already hide them are found; `self` keeps you from hiding your own messages.
 */
export function ignoreCommand(
  name: 'ignore' | 'unignore',
  arg: string,
  list: readonly string[],
  context: { person?: IgnoreIdentity; self?: IgnoreIdentity } = {},
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
  if (/\s|\0/.test(target)) return { notice: `Usage: /${name} <nick>` }
  if (target.length > notificationLimits.ignoreEntry)
    return { notice: `That’s too long; use at most ${notificationLimits.ignoreEntry} characters.` }
  const person = context.person ?? { nick: target }
  if (name === 'ignore') {
    if (hasIgnore(list, target)) return { notice: `You’re already ignoring ${target}.` }
    const self = context.self
    if (self && maskMatches(target, self.nick, self.user, self.host))
      return { notice: 'That would hide your own messages.' }
    const by = covering(list, person)
    if (by.length) return { notice: `${target} is already hidden by ${by.join(', ')}.` }
    if (list.length >= notificationLimits.ignore)
      return {
        notice: `You’re ignoring the maximum of ${notificationLimits.ignore}; remove someone in Settings → Messages & media first.`,
      }
    return {
      ignore: addIgnore(list, target),
      notice: `Ignoring ${target}. Their messages are hidden; /unignore ${target} to undo.`,
    }
  }
  if (hasIgnore(list, target))
    return { ignore: removeIgnore(list, target), notice: `No longer ignoring ${target}.` }
  const by = covering(list, person)
  if (by.length)
    return {
      notice: `${target} is hidden by ${by.join(', ')}; /unignore ${by[0]} to stop.`,
    }
  return { notice: `You aren’t ignoring ${target}.` }
}
