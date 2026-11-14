// Which slash commands the composer's menu offers for what has been typed after the “/”.

export interface CommandSpec {
  name: string
  args: string
  description: string
  fields: readonly CommandArgumentSpec[]
  /** Only meaningful inside a channel. */
  channel?: boolean
  /** Only offered to channel operators and above (op, admin, owner). */
  operator?: boolean
  /** Left out of the menu until at least `HIDDEN_COMMAND_LETTERS` letters are typed. */
  hidden?: boolean
}

export interface CommandArgumentSpec {
  name: string
  type: 'text' | 'message' | 'nickname' | 'channel' | 'emoji' | 'duration' | 'secret'
  required?: boolean
  label: string
  hint: string
  placeholder?: string
  maxBytes?: number
  choices?: readonly { value: string; label: string }[]
  autocomplete?: 'nickname' | 'channel' | 'emoji' | 'ignored'
  token?: boolean
}

const field = (
  name: string,
  type: CommandArgumentSpec['type'],
  options: Partial<CommandArgumentSpec> = {},
): CommandArgumentSpec => ({
  name,
  type,
  label: `workspaceUi.commandEditor.fields.${name}`,
  hint: `workspaceUi.commandEditor.hints.${name}`,
  ...options,
})
const nick = () => field('nickname', 'nickname', { required: true, token: true })
const reason = () => field('reason', 'message')

/** The single catalog used by autocomplete, guided entry, and execution. */
export const commands: CommandSpec[] = [
  {
    name: 'me',
    args: '<action>',
    description: 'Describe what you’re doing',
    fields: [field('action', 'message', { required: true })],
  },
  {
    name: 'slap',
    args: '<nick>',
    description: 'Slap someone around a bit with a large cucumber',
    fields: [nick()],
  },
  {
    name: 'msg',
    args: '<nick> <message>',
    description: 'Send someone a private message',
    fields: [nick(), field('message', 'message', { required: true })],
  },
  { name: 'query', args: '<nick>', description: 'Open a private conversation', fields: [nick()] },
  {
    name: 'join',
    args: '<#channel> [key]',
    description: 'Join a channel',
    fields: [
      field('channel', 'channel', { required: true, token: true }),
      field('key', 'secret', { token: true }),
    ],
  },
  {
    name: 'part',
    args: '[reason]',
    description: 'Leave this channel',
    channel: true,
    fields: [reason()],
  },
  {
    name: 'nick',
    args: '<new nick>',
    description: 'Change your nickname on this network',
    fields: [field('nickname', 'text', { required: true, token: true })],
  },
  {
    name: 'topic',
    args: '[new topic]',
    description: 'Show or change the channel topic',
    channel: true,
    fields: [field('topic', 'message')],
  },
  {
    name: 'away',
    args: '[message]',
    description: 'Mark yourself as away',
    fields: [field('message', 'message')],
  },
  {
    name: 'status',
    args: '[emoji] <message> [--for <duration>]',
    description: 'Set your status',
    fields: [
      field('emoji', 'emoji', { token: true, maxBytes: 64 }),
      field('message', 'message', { maxBytes: 200 }),
      field('duration', 'duration', { token: true }),
    ],
  },
  {
    name: 'afk',
    args: '[reason]',
    description: 'Mark yourself as away from keyboard until you next send a message',
    fields: [reason()],
  },
  {
    name: 'brb',
    args: '[reason]',
    description: 'Mark yourself as right back until you next send a message',
    fields: [reason()],
  },
  { name: 'back', args: '', description: 'Mark yourself as back', fields: [] },
  {
    name: 'op',
    args: '<nick>',
    description: 'Make someone a channel operator',
    channel: true,
    fields: [nick()],
  },
  {
    name: 'deop',
    args: '<nick>',
    description: 'Remove operator status',
    channel: true,
    fields: [nick()],
  },
  {
    name: 'voice',
    args: '<nick>',
    description: 'Give someone voice',
    channel: true,
    fields: [nick()],
  },
  { name: 'devoice', args: '<nick>', description: 'Remove voice', channel: true, fields: [nick()] },
  {
    name: 'kick',
    args: '<nick> [reason]',
    description: 'Remove someone from the channel',
    channel: true,
    fields: [nick(), reason()],
  },
  {
    name: 'ban',
    args: '<nick>',
    description: 'Ban someone from the channel',
    channel: true,
    fields: [field('mask', 'text', { required: true, token: true, autocomplete: 'nickname' })],
  },
  {
    name: 'invite',
    args: '<nick> [#channel]',
    description: 'Invite someone to a channel',
    fields: [nick(), field('channel', 'channel', { token: true })],
  },
  {
    name: 'mode',
    args: '<modes> [args]',
    description: 'Change channel or user modes',
    fields: [
      field('modes', 'text', { required: true, token: true }),
      field('arguments', 'text', { autocomplete: 'nickname' }),
    ],
  },
  {
    name: 'whois',
    args: '<nick>',
    description: 'Look someone up (shown in the server window)',
    fields: [nick()],
  },
  {
    name: 'ignore',
    args: '[nick]',
    description: 'Hide someone’s messages, or show who you ignore',
    fields: [field('mask', 'text', { token: true, autocomplete: 'nickname' })],
  },
  {
    name: 'unignore',
    args: '<nick>',
    description: 'Stop ignoring someone',
    fields: [field('mask', 'text', { required: true, token: true, autocomplete: 'ignored' })],
  },
  {
    name: 'list',
    args: '[filter]',
    description: 'Browse channels on this network',
    fields: [field('filter', 'text')],
  },
  { name: 'server', args: '', description: 'Open this network’s server window', fields: [] },
  {
    name: 'settings',
    args: '',
    description: 'Channel settings: topic, modes and bans',
    channel: true,
    fields: [],
  },
  { name: 'banlist', args: '', description: 'Show this channel’s bans', channel: true, fields: [] },
  {
    name: 'gif',
    args: '<search terms>',
    description: 'Send a random matching GIF',
    fields: [field('query', 'message', { required: true })],
  },
  {
    name: 'quote',
    args: '<raw IRC line>',
    description: 'Send a raw IRC command',
    fields: [field('line', 'message', { required: true })],
  },
  {
    name: 'attract',
    args: '',
    description: 'Make your last message sparkle for everyone here',
    channel: true,
    operator: true,
    hidden: true,
    fields: [],
  },
]

export const commandAliases: Readonly<Record<string, string>> = {
  raw: 'quote',
  j: 'join',
  leave: 'part',
  q: 'query',
  w: 'whois',
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
export const awayLabel = (message?: string, fallback = 'away') => message?.trim() || fallback

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
