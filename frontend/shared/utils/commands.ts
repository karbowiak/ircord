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
