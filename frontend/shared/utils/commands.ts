// Which slash commands the composer's menu offers for what has been typed after the “/”.

export interface CommandSpec {
  name: string
  args: string
  description: string
  /** Only meaningful inside a channel. */
  channel?: boolean
  /** Only offered to channel operators and above (op, admin, owner). */
  operator?: boolean
  /** Left out of the menu until at least `HIDDEN_COMMAND_LETTERS` letters are typed. */
  hidden?: boolean
}

/** The action `/slap <nick>` sends, as `/me` would: “* you slaps nick around a bit with a large cucumber”. */
export const slapAction = (nick: string) => `slaps ${nick} around a bit with a large cucumber`

/**
 * The AWAY text `/afk` and `/brb` send: “AFK” or “AFK: reason”. It is plain IRC away, so every
 * client shows it; the backend ends it with your next message.
 */
export const briefAwayText = (kind: 'afk' | 'brb', reason: string) =>
  reason ? `${kind.toUpperCase()}: ${reason}` : kind.toUpperCase()

/** Whether an away text is one `/afk` or `/brb` set, which your next message ends. */
export const isBriefAway = (message?: string) => /^(afk|brb)(:|$)/i.test(message ?? '')

/** What your own presence line says while away: the away text, or just “away”. */
export const awayLabel = (message?: string) => message?.trim() || 'away'

/** `/attr` shows `/attract`; `/`, `/a`, `/at` and `/att` do not. */
export const HIDDEN_COMMAND_LETTERS = 4

export function matchCommands(
  commands: readonly CommandSpec[],
  query: string,
  where: { channel: boolean; operator: boolean },
) {
  const q = query.toLowerCase()
  return commands.filter(
    (c) =>
      c.name.startsWith(q) &&
      (!c.channel || where.channel) &&
      (!c.operator || where.operator) &&
      (!c.hidden || q.length >= HIDDEN_COMMAND_LETTERS),
  )
}
