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
  context: {
    person?: IgnoreIdentity
    self?: IgnoreIdentity
    translate?: (key: string, values?: Record<string, string | number>, plural?: number) => string
  } = {},
): IgnoreOutcome {
  const message = (
    key: string,
    fallback: string,
    values?: Record<string, string | number>,
    plural?: number,
  ) => context.translate?.(`workspaceUi.commands.${key}`, values, plural) ?? fallback
  const target = arg.trim().replace(/^@/, '')
  if (!target) {
    if (name === 'unignore')
      return { notice: message('ignoreUsage', 'Usage: /unignore <nick>', { command: name }) }
    return {
      notice: list.length
        ? message(
            'ignoreList',
            `Ignoring ${list.length === 1 ? '1 person' : `${list.length} people`}: ${list.join(', ')}`,
            { count: list.length, names: list.join(', ') },
            list.length,
          )
        : message('ignoreEmpty', 'You aren’t ignoring anyone. Use /ignore <nick> to start.'),
    }
  }
  if (/\s|\0/.test(target))
    return { notice: message('ignoreUsage', `Usage: /${name} <nick>`, { command: name }) }
  if (target.length > notificationLimits.ignoreEntry)
    return {
      notice: message(
        'ignoreTooLong',
        `That’s too long; use at most ${notificationLimits.ignoreEntry} characters.`,
        { limit: notificationLimits.ignoreEntry },
      ),
    }
  const person = context.person ?? { nick: target }
  if (name === 'ignore') {
    if (hasIgnore(list, target))
      return {
        notice: message('alreadyIgnoring', `You’re already ignoring ${target}.`, { target }),
      }
    const self = context.self
    if (self && maskMatches(target, self.nick, self.user, self.host))
      return { notice: message('ignoreSelf', 'That would hide your own messages.') }
    const by = covering(list, person)
    if (by.length)
      return {
        notice: message('hiddenBy', `${target} is already hidden by ${by.join(', ')}.`, {
          target,
          masks: by.join(', '),
        }),
      }
    if (list.length >= notificationLimits.ignore)
      return {
        notice: message(
          'ignoreMaximum',
          `You’re ignoring the maximum of ${notificationLimits.ignore}; remove someone in Settings → Chat first.`,
          { limit: notificationLimits.ignore },
        ),
      }
    return {
      ignore: addIgnore(list, target),
      notice: message(
        'ignoreAdded',
        `Ignoring ${target}. Their messages are hidden; /unignore ${target} to undo.`,
        { target },
      ),
    }
  }
  if (hasIgnore(list, target))
    return {
      ignore: removeIgnore(list, target),
      notice: message('ignoreRemoved', `No longer ignoring ${target}.`, { target }),
    }
  const by = covering(list, person)
  if (by.length)
    return {
      notice: message(
        'ignoreCovered',
        `${target} is hidden by ${by.join(', ')}; /unignore ${by[0]} to stop.`,
        { target, masks: by.join(', '), first: by[0]! },
      ),
    }
  return { notice: message('notIgnoring', `You aren’t ignoring ${target}.`, { target }) }
}
