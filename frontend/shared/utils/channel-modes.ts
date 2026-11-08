// Channel modes, as far as a client can know them. ISUPPORT CHANMODES says which letters exist
// and how they take parameters; what a letter *means* is up to the server software, so common
// letters get descriptions and the rest are shown as server-specific.

/** CHANMODES=A,B,C,D: list modes, always-parameter, parameter-when-set, and flags. */
export function parseChanmodes(isupport: Record<string, string> | undefined) {
  const [list = 'beI', always = 'k', whenSet = 'l', flags = 'imnpst'] = (
    isupport?.CHANMODES ?? 'beI,k,l,imnpst'
  ).split(',')
  return { list, always, whenSet, flags }
}

export interface ModeInfo {
  label: string
  description: string
  /** The letter means different things on different server software. */
  varies?: boolean
}

/** Flags and parameter modes. Descriptions follow the most common meaning (Solanum, InspIRCd, UnrealIRCd). */
export const modeInfo: Record<string, ModeInfo> = {
  n: { label: 'No outside messages', description: 'Only people in the channel can send to it.' },
  t: { label: 'Topic lock', description: 'Only operators can change the topic.' },
  m: { label: 'Moderated', description: 'Only voiced people and operators can talk.' },
  i: {
    label: 'Invite only',
    description: 'People need an invite (or an invite exception) to join.',
  },
  s: { label: 'Secret', description: 'Hidden from the channel list and WHOIS.' },
  p: { label: 'Private', description: 'Hidden from WHOIS; listed without its topic.' },
  k: { label: 'Key', description: 'A password needed to join.' },
  l: { label: 'User limit', description: 'Nobody else can join once this many people are in.' },
  r: {
    label: 'Registered only',
    description: 'Only people logged in to services can join.',
    varies: true,
  },
  R: {
    label: 'Registered only',
    description: 'Only people logged in to services can join.',
    varies: true,
  },
  c: { label: 'No colours', description: 'Colour codes are blocked or stripped.', varies: true },
  C: {
    label: 'No CTCP',
    description: 'CTCP messages (other than ACTION) are blocked.',
    varies: true,
  },
  S: {
    label: 'TLS only / strip colours',
    description: 'Only TLS connections may join, or colours are stripped, depending on the server.',
    varies: true,
  },
  T: { label: 'No notices', description: 'Notices to the channel are blocked.', varies: true },
  z: {
    label: 'Reduced moderation',
    description: 'Blocked messages are shown to operators instead of dropped.',
    varies: true,
  },
  g: {
    label: 'Free invite',
    description: 'Anyone in the channel can invite others.',
    varies: true,
  },
  P: { label: 'Permanent', description: 'The channel stays when everyone leaves.', varies: true },
  Q: {
    label: 'No forwarding',
    description: 'Users can’t be forwarded into this channel.',
    varies: true,
  },
  F: {
    label: 'Free forward target',
    description: 'Anyone can forward users to this channel.',
    varies: true,
  },
  u: {
    label: 'Unfiltered',
    description: 'Operators see messages the server would otherwise filter.',
    varies: true,
  },
  f: {
    label: 'Forward',
    description: 'Where people go when they can’t join, or a flood limit, depending on the server.',
    varies: true,
  },
  j: {
    label: 'Join throttle',
    description: 'Limits how many joins are allowed per interval (joins:seconds).',
    varies: true,
  },
  L: { label: 'Large ban list', description: 'Allows longer ban lists.', varies: true },
}

/**
 * List modes (type A). `description` is the one-line purpose; `overrides` says what the list does
 * and doesn't override, so bans, ban exceptions and invite exceptions don't read as the same list.
 */
export const listInfo: Record<
  string,
  { label: string; singular: string; description: string; overrides?: string }
> = {
  b: {
    label: 'Bans',
    singular: 'ban',
    description: 'People matching a ban can’t join.',
    overrides:
      'A ban doesn’t remove anyone already here; kick them too. Ban exceptions override bans.',
  },
  e: {
    label: 'Ban exceptions',
    singular: 'exception',
    description: 'People matching an exception can join even when a ban matches them.',
    overrides:
      'It lets them past bans (on some networks, quiets too). Invite-only (+i), the key and the user limit still apply to them.',
  },
  I: {
    label: 'Invite exceptions',
    singular: 'invite exception',
    description:
      'People matching an invite exception can join an invite-only channel without an invite.',
    overrides: 'It overrides invite-only (+i) only: a ban still keeps them out.',
  },
  q: { label: 'Quiets', singular: 'quiet', description: 'People matching a quiet can’t talk.' },
}

// ---- Masks ------------------------------------------------------------------------------------
// Verified against Ergo (v2.19): `alice` is stored as `alice!*@*`, `bob@host` as `*!bob@host`,
// `carol!id` as `carol!id@*`, and an extended mask the network doesn't advertise is read as a
// nickname (`$a:erin` → `$a:erin!*@*`). Expanding the same way here shows what will be stored.

/** ISUPPORT EXTBAN=<prefix>,<types>: extended masks such as `$a:account` (Solanum) or `m:` (Ergo). */
export interface Extbans {
  prefix: string
  types: string
}
export function parseExtbans(isupport: Record<string, string> | undefined): Extbans | undefined {
  const value = isupport?.EXTBAN
  if (value === undefined) return undefined
  const [prefix = '', types = ''] = value.includes(',') ? value.split(',') : ['', value]
  return { prefix, types }
}

/**
 * The account extban, where it's verified: `$a:account` on Solanum/Charybdis-style servers and
 * `~a:account` on UnrealIRCd. Other servers (Ergo, InspIRCd) use different or no letters.
 */
export const accountExtban = (x: Extbans | undefined) =>
  x && (x.prefix === '$' || x.prefix === '~') && x.types.includes('a') ? `${x.prefix}a:` : undefined
/** The mute extban `m:<mask>` (Ergo, InspIRCd): matching people can stay but can't talk. */
export const muteExtban = (x: Extbans | undefined) =>
  x && x.prefix === '' && x.types.includes('m') ? 'm:' : undefined

/**
 * An extended mask the network advertises, split into its type letter and argument. On `$`
 * networks (Solanum/Charybdis) `$~a` is the negated form: everyone *not* matching `$a`.
 */
export function extbanOf(value: string, x: Extbans | undefined) {
  if (!x?.types) return undefined
  const p = x.prefix
  if (!value.startsWith(p)) return undefined
  const negated = p === '$' && value[p.length] === '~'
  const start = p.length + (negated ? 1 : 0)
  const type = value[start]
  if (!type || !x.types.includes(type)) return undefined
  const rest = value.slice(start + 1)
  // Prefixless extbans need the colon (`m:alice`), or every nick starting with m would match.
  if (rest.startsWith(':')) return { type, arg: rest.slice(1), negated }
  if (p && !rest) return { type, arg: '', negated }
  return undefined
}

/**
 * Solanum's ban forwarding, `mask$#channel`: matching people are sent to #channel instead.
 * Only `$` networks have it; the target is split off so the mask part can still be expanded.
 */
function splitForward(value: string, x: Extbans | undefined) {
  if (x?.prefix !== '$') return { mask: value, forward: '' }
  const at = value.indexOf('$#')
  return at > 0
    ? { mask: value.slice(0, at), forward: value.slice(at + 1) }
    : { mask: value, forward: '' }
}

/** Fills the missing parts of `nick!user@host` with `*`, as servers do. */
export function expandMask(value: string) {
  const v = value.trim()
  const at = v.lastIndexOf('@')
  const left = at >= 0 ? v.slice(0, at) : v
  const host = at >= 0 ? v.slice(at + 1) : ''
  const bang = left.indexOf('!')
  // `bob@host` names a user, not a nick; `carol!id` has no host.
  const nick = bang >= 0 ? left.slice(0, bang) : at >= 0 ? '' : left
  const user = bang >= 0 ? left.slice(bang + 1) : at >= 0 ? left : ''
  return `${nick || '*'}!${user || '*'}@${host || '*'}`
}

/**
 * What is sent for typed text. With the network's EXTBAN known, anything starting with its
 * extended-mask prefix goes as typed (only the prefixless mute extban's mask is expanded, as Ergo
 * does), a `$#channel` forward is kept after the expanded mask, and anything else is a mask.
 * Without EXTBAN, text containing `$` is left alone because the client can't tell what it means.
 */
export function toMask(value: string, extbans?: Extbans) {
  const v = value.trim()
  if (!extbans) return v.includes('$') ? v : expandMask(v)
  const x = extbanOf(v, extbans)
  if (x) return x.type === 'm' && !extbans.prefix ? `m:${expandMask(x.arg)}` : v
  if (extbans.prefix && v.startsWith(extbans.prefix)) return v
  const { mask, forward } = splitForward(v, extbans)
  return forward ? `${expandMask(mask)}$${forward}` : expandMask(v)
}

/** IRC wildcard matching: `*` is any run of characters, `?` exactly one; case is ignored. */
export function maskMatches(mask: string, identity: string) {
  const pattern = mask
    .toLowerCase()
    .replace(/[.+^${}()|[\]\\]/g, '\\$&')
    .replace(/\*/g, '.*')
    .replace(/\?/g, '.')
  return new RegExp(`^${pattern}$`, 's').test(identity.toLowerCase())
}

const onlyWildcards = (part: string) => /^[*?]*$/.test(part)

export interface MaskPreview {
  /** What Add sends; empty when there's nothing to send. */
  mask: string
  /** Why the text can't be sent, if it can't. */
  error?: string
  /** The typed text was shorthand and was expanded. */
  expanded: boolean
  /** Extended mask kind, when the network advertises it. */
  extban?: 'account' | 'mute' | 'other'
  /** What the mask covers, in plain words. */
  scope?: string
  /** Things worth knowing before adding it (broad masks, unsupported extended masks). */
  warnings: string[]
}

/**
 * Why a `nick!user@host` mask is broad, if it is. Judged by what the mask pins down rather than
 * by listing shapes: a host with at most one literal label (`*.com`, `203.*`), a country domain
 * (`*.co.uk`), or a nick of one or two letters plus a wildcard with nothing else pinned.
 */
export function broadness(mask: string): string | undefined {
  const [nick = '*', rest = '*@*'] = mask.split('!')
  const [user = '*', host = '*'] = rest.split('@')
  if (onlyWildcards(nick) && onlyWildcards(user) && onlyWildcards(host))
    return 'This matches everyone, including you.'
  if (!onlyWildcards(user)) return undefined
  if (onlyWildcards(nick) && /[*?]/.test(host)) {
    const literal = host.split('.').filter((label) => !onlyWildcards(label))
    const countryDomain =
      literal.length === 2 &&
      /^[a-z]{2}$/i.test(literal[1]!) &&
      literal[0]!.replace(/[*?]/g, '').length <= 3
    const ipPrefix = literal.length <= 2 && literal.every((label) => /^\d+$/.test(label))
    if (literal.length <= 1 || countryDomain || ipPrefix)
      return 'The host part is very broad: it can match a whole country, provider or address range.'
  }
  if (onlyWildcards(host) && /[*?]/.test(nick) && nick.replace(/[*?]/g, '').length <= 2)
    return 'The nick part is very broad: it matches every nick with those few letters.'
  return undefined
}

/** Explains what typed text becomes before it's added to a list. */
export function previewMask(value: string, extbans?: Extbans): MaskPreview {
  const v = value.trim()
  const warnings: string[] = []
  if (!v) return { mask: '', expanded: false, warnings }
  if (/\s/.test(v))
    return { mask: '', error: 'Masks can’t contain spaces.', expanded: false, warnings }
  if (v.startsWith(':') || v.includes(','))
    return {
      mask: '',
      error: 'Masks can’t start with “:” or contain commas.',
      expanded: false,
      warnings,
    }
  const mask = toMask(v, extbans)
  const x = extbans ? extbanOf(v, extbans) : undefined
  const expanded = mask !== v
  if (x) {
    const account = accountExtban(extbans)
    if (account && `${extbans!.prefix}${x.type}:` === account) {
      const not = x.negated ? 'not ' : ''
      return {
        mask,
        expanded,
        extban: 'account',
        warnings,
        scope: x.arg
          ? `Anyone ${not}logged in to the account ${x.arg}, whatever nick they use.`
          : `Anyone ${not}logged in to an account.`,
      }
    }
    if (x.type === 'm' && muteExtban(extbans)) {
      const inner = mask.slice(2)
      const broad = broadness(inner)
      if (broad) warnings.push(broad)
      return {
        mask,
        expanded,
        extban: 'mute',
        warnings,
        scope: `Mutes ${describeParts(inner)}: they can stay in the channel but can’t talk.`,
      }
    }
    return {
      mask,
      expanded,
      extban: 'other',
      warnings,
      scope: `A server-specific extended mask${x.negated ? ' (negated: it matches everyone the rest doesn’t)' : ''}, sent as typed.`,
    }
  }
  if (!extbans && v.includes('$'))
    return {
      mask,
      expanded,
      warnings: [
        'Sent as typed: this network doesn’t advertise extended masks (EXTBAN), so it may not understand it.',
      ],
    }
  if (extbans?.prefix && v.startsWith(extbans.prefix))
    return {
      mask,
      expanded,
      extban: 'other',
      warnings: [
        `This network doesn’t advertise this ${extbans.prefix} mask type; it’s sent as typed and the server decides.`,
      ],
    }
  if (extbans && /^[$~]/.test(v))
    warnings.push(
      `This network doesn’t offer ${v[0]}… extended masks, so it reads this as a nickname.`,
    )
  const { mask: plain, forward } = splitForward(mask, extbans)
  const broad = broadness(plain)
  if (broad) warnings.push(broad)
  return {
    mask,
    expanded,
    warnings,
    scope: `Matches ${describeParts(plain)}${forward ? ` and sends them to ${forward} instead` : ''}.`,
  }
}

/** `nick!user@host` in words: which of the three parts the mask actually pins down. */
function describeParts(mask: string) {
  const [nick = '*', rest = '*@*'] = mask.split('!')
  const [user = '*', host = '*'] = rest.split('@')
  const parts: string[] = []
  if (!onlyWildcards(nick))
    parts.push(
      `${nick.includes('*') || nick.includes('?') ? 'a nick matching' : 'the nick'} ${nick}`,
    )
  if (!onlyWildcards(user))
    parts.push(
      `${user.includes('*') || user.includes('?') ? 'a username matching' : 'the username'} ${user}`,
    )
  if (!onlyWildcards(host))
    parts.push(
      `${host.includes('*') || host.includes('?') ? 'a host matching' : 'the host'} ${host}`,
    )
  return parts.length ? `anyone with ${parts.join(', and ')}` : 'everyone'
}
