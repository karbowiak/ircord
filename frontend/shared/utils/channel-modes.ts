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

export type ChannelModeTranslator = (
  key: string,
  values?: Record<string, string | number>,
) => string

const englishMessages: Record<string, string> = {
  'collectionsUi.channel.mode.n.label': 'No outside messages',
  'collectionsUi.channel.mode.n.description': 'Only people in the channel can send to it.',
  'collectionsUi.channel.mode.t.label': 'Topic lock',
  'collectionsUi.channel.mode.t.description': 'Only operators can change the topic.',
  'collectionsUi.channel.mode.m.label': 'Moderated',
  'collectionsUi.channel.mode.m.description': 'Only voiced people and operators can talk.',
  'collectionsUi.channel.mode.i.label': 'Invite only',
  'collectionsUi.channel.mode.i.description':
    'People need an invite (or an invite exception) to join.',
  'collectionsUi.channel.mode.s.label': 'Secret',
  'collectionsUi.channel.mode.s.description': 'Hidden from the channel list and WHOIS.',
  'collectionsUi.channel.mode.p.label': 'Private',
  'collectionsUi.channel.mode.p.description': 'Hidden from WHOIS; listed without its topic.',
  'collectionsUi.channel.mode.k.label': 'Key',
  'collectionsUi.channel.mode.k.description': 'A password needed to join.',
  'collectionsUi.channel.mode.l.label': 'User limit',
  'collectionsUi.channel.mode.l.description': 'Nobody else can join once this many people are in.',
  'collectionsUi.channel.mode.r.label': 'Registered only',
  'collectionsUi.channel.mode.r.description': 'Only people logged in to services can join.',
  'collectionsUi.channel.mode.R.label': 'Registered only',
  'collectionsUi.channel.mode.R.description': 'Only people logged in to services can join.',
  'collectionsUi.channel.mode.c.label': 'No colours',
  'collectionsUi.channel.mode.c.description': 'Colour codes are blocked or stripped.',
  'collectionsUi.channel.mode.C.label': 'No CTCP',
  'collectionsUi.channel.mode.C.description': 'CTCP messages (other than ACTION) are blocked.',
  'collectionsUi.channel.mode.S.label': 'TLS only / strip colours',
  'collectionsUi.channel.mode.S.description':
    'Only TLS connections may join, or colours are stripped, depending on the server.',
  'collectionsUi.channel.mode.T.label': 'No notices',
  'collectionsUi.channel.mode.T.description': 'Notices to the channel are blocked.',
  'collectionsUi.channel.mode.z.label': 'Reduced moderation',
  'collectionsUi.channel.mode.z.description':
    'Blocked messages are shown to operators instead of dropped.',
  'collectionsUi.channel.mode.g.label': 'Free invite',
  'collectionsUi.channel.mode.g.description': 'Anyone in the channel can invite others.',
  'collectionsUi.channel.mode.P.label': 'Permanent',
  'collectionsUi.channel.mode.P.description': 'The channel stays when everyone leaves.',
  'collectionsUi.channel.mode.Q.label': 'No forwarding',
  'collectionsUi.channel.mode.Q.description': 'Users can’t be forwarded into this channel.',
  'collectionsUi.channel.mode.F.label': 'Free forward target',
  'collectionsUi.channel.mode.F.description': 'Anyone can forward users to this channel.',
  'collectionsUi.channel.mode.u.label': 'Unfiltered',
  'collectionsUi.channel.mode.u.description':
    'Operators see messages the server would otherwise filter.',
  'collectionsUi.channel.mode.f.label': 'Forward',
  'collectionsUi.channel.mode.f.description':
    'Where people go when they can’t join, or a flood limit, depending on the server.',
  'collectionsUi.channel.mode.j.label': 'Join throttle',
  'collectionsUi.channel.mode.j.description':
    'Limits how many joins are allowed per interval (joins:seconds).',
  'collectionsUi.channel.mode.L.label': 'Large ban list',
  'collectionsUi.channel.mode.L.description': 'Allows longer ban lists.',
  'collectionsUi.channel.list.b.label': 'Bans',
  'collectionsUi.channel.list.b.singular': 'ban',
  'collectionsUi.channel.list.b.description': 'People matching a ban can’t join.',
  'collectionsUi.channel.list.b.overrides':
    'A ban doesn’t remove anyone already here; kick them too. Ban exceptions override bans.',
  'collectionsUi.channel.list.e.label': 'Ban exceptions',
  'collectionsUi.channel.list.e.singular': 'exception',
  'collectionsUi.channel.list.e.description':
    'People matching an exception can join even when a ban matches them.',
  'collectionsUi.channel.list.e.overrides':
    'It lets them past bans (on some networks, quiets too). Invite-only (+i), the key and the user limit still apply to them.',
  'collectionsUi.channel.list.I.label': 'Invite exceptions',
  'collectionsUi.channel.list.I.singular': 'invite exception',
  'collectionsUi.channel.list.I.description':
    'People matching an invite exception can join an invite-only channel without an invite.',
  'collectionsUi.channel.list.I.overrides':
    'It overrides invite-only (+i) only: a ban still keeps them out.',
  'collectionsUi.channel.list.q.label': 'Quiets',
  'collectionsUi.channel.list.q.singular': 'quiet',
  'collectionsUi.channel.list.q.description': 'People matching a quiet can’t talk.',
  'collectionsUi.channel.mask.broad.everyone': 'This matches everyone, including you.',
  'collectionsUi.channel.mask.broad.host':
    'The host part is very broad: it can match a whole country, provider or address range.',
  'collectionsUi.channel.mask.broad.nick':
    'The nick part is very broad: it matches every nick with those few letters.',
  'collectionsUi.channel.mask.invalidSpaces': 'Masks can’t contain spaces.',
  'collectionsUi.channel.mask.invalidPrefix': 'Masks can’t start with “:” or contain commas.',
  'collectionsUi.channel.mask.account':
    'Anyone logged in to the account {account}, whatever nick they use.',
  'collectionsUi.channel.mask.accountNot':
    'Anyone not logged in to the account {account}, whatever nick they use.',
  'collectionsUi.channel.mask.anyAccount': 'Anyone logged in to an account.',
  'collectionsUi.channel.mask.anyAccountNot': 'Anyone not logged in to an account.',
  'collectionsUi.channel.mask.mute': 'Mutes {parts}: they can stay in the channel but can’t talk.',
  'collectionsUi.channel.mask.extended': 'A server-specific extended mask, sent as typed.',
  'collectionsUi.channel.mask.extendedNegated':
    'A server-specific extended mask (negated: it matches everyone the rest doesn’t), sent as typed.',
  'collectionsUi.channel.mask.unknownExtban':
    'Sent as typed: this network doesn’t advertise extended masks (EXTBAN), so it may not understand it.',
  'collectionsUi.channel.mask.unknownType':
    'This network doesn’t advertise this {prefix} mask type; it’s sent as typed and the server decides.',
  'collectionsUi.channel.mask.noExtendedType':
    'This network doesn’t offer {prefix}… extended masks, so it reads this as a nickname.',
  'collectionsUi.channel.mask.matches': 'Matches {parts}.',
  'collectionsUi.channel.mask.matchesForward':
    'Matches {parts} and sends them to {channel} instead.',
  'collectionsUi.channel.mask.nick': 'the nick {value}',
  'collectionsUi.channel.mask.nickPattern': 'a nick matching {value}',
  'collectionsUi.channel.mask.username': 'the username {value}',
  'collectionsUi.channel.mask.usernamePattern': 'a username matching {value}',
  'collectionsUi.channel.mask.host': 'the host {value}',
  'collectionsUi.channel.mask.hostPattern': 'a host matching {value}',
  'collectionsUi.channel.mask.anyoneWith': 'anyone with {parts}',
  'collectionsUi.channel.mask.everyone': 'everyone',
  'collectionsUi.channel.mask.joinTwo': '{first} and {second}',
  'collectionsUi.channel.mask.joinMany': '{first}, and {last}',
}

const english: ChannelModeTranslator = (key, values) =>
  Object.entries(values ?? {}).reduce(
    (message, [name, value]) => message.replaceAll('{' + name + '}', String(value)),
    englishMessages[key] ?? key,
  )

/** Common flags and parameter modes, described by the most common server meanings. */
const modeKeys: Record<string, { varies?: boolean }> = {
  n: {},
  t: {},
  m: {},
  i: {},
  s: {},
  p: {},
  k: {},
  l: {},
  r: { varies: true },
  R: { varies: true },
  c: { varies: true },
  C: { varies: true },
  S: { varies: true },
  T: { varies: true },
  z: { varies: true },
  g: { varies: true },
  P: { varies: true },
  Q: { varies: true },
  F: { varies: true },
  u: { varies: true },
  f: { varies: true },
  j: { varies: true },
  L: { varies: true },
}

export function modeInfo(translate: ChannelModeTranslator = english): Record<string, ModeInfo> {
  return Object.fromEntries(
    Object.entries(modeKeys).map(([mode, info]) => [
      mode,
      {
        ...info,
        label: translate(`collectionsUi.channel.mode.${mode}.label`),
        description: translate(`collectionsUi.channel.mode.${mode}.description`),
      },
    ]),
  )
}

/**
 * List modes (type A). `description` is the one-line purpose; `overrides` says what the list does
 * and doesn't override, so bans, ban exceptions and invite exceptions don't read as the same list.
 */
const listModes = ['b', 'e', 'I', 'q'] as const
export function listInfo(
  translate: ChannelModeTranslator = english,
): Record<string, { label: string; singular: string; description: string; overrides?: string }> {
  return Object.fromEntries(
    listModes.map((mode) => {
      const key = `collectionsUi.channel.list.${mode}`
      return [
        mode,
        {
          label: translate(`${key}.label`),
          singular: translate(`${key}.singular`),
          description: translate(`${key}.description`),
          ...(mode === 'b' || mode === 'e' || mode === 'I'
            ? { overrides: translate(`${key}.overrides`) }
            : {}),
        },
      ]
    }),
  )
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
export function broadness(
  mask: string,
  translate: ChannelModeTranslator = english,
): string | undefined {
  const [nick = '*', rest = '*@*'] = mask.split('!')
  const [user = '*', host = '*'] = rest.split('@')
  if (onlyWildcards(nick) && onlyWildcards(user) && onlyWildcards(host))
    return translate('collectionsUi.channel.mask.broad.everyone')
  if (!onlyWildcards(user)) return undefined
  if (onlyWildcards(nick) && /[*?]/.test(host)) {
    const literal = host.split('.').filter((label) => !onlyWildcards(label))
    const countryDomain =
      literal.length === 2 &&
      /^[a-z]{2}$/i.test(literal[1]!) &&
      literal[0]!.replace(/[*?]/g, '').length <= 3
    const ipPrefix = literal.length <= 2 && literal.every((label) => /^\d+$/.test(label))
    if (literal.length <= 1 || countryDomain || ipPrefix)
      return translate('collectionsUi.channel.mask.broad.host')
  }
  if (onlyWildcards(host) && /[*?]/.test(nick) && nick.replace(/[*?]/g, '').length <= 2)
    return translate('collectionsUi.channel.mask.broad.nick')
  return undefined
}

/** Explains what typed text becomes before it's added to a list. */
export function previewMask(
  value: string,
  extbans?: Extbans,
  translate: ChannelModeTranslator = english,
): MaskPreview {
  const v = value.trim()
  const warnings: string[] = []
  if (!v) return { mask: '', expanded: false, warnings }
  if (/\s/.test(v))
    return {
      mask: '',
      error: translate('collectionsUi.channel.mask.invalidSpaces'),
      expanded: false,
      warnings,
    }
  if (v.startsWith(':') || v.includes(','))
    return {
      mask: '',
      error: translate('collectionsUi.channel.mask.invalidPrefix'),
      expanded: false,
      warnings,
    }
  const mask = toMask(v, extbans)
  const x = extbans ? extbanOf(v, extbans) : undefined
  const expanded = mask !== v
  if (x) {
    const account = accountExtban(extbans)
    if (account && `${extbans!.prefix}${x.type}:` === account) {
      return {
        mask,
        expanded,
        extban: 'account',
        warnings,
        scope: x.arg
          ? translate(
              x.negated
                ? 'collectionsUi.channel.mask.accountNot'
                : 'collectionsUi.channel.mask.account',
              { account: x.arg },
            )
          : translate(
              x.negated
                ? 'collectionsUi.channel.mask.anyAccountNot'
                : 'collectionsUi.channel.mask.anyAccount',
            ),
      }
    }
    if (x.type === 'm' && muteExtban(extbans)) {
      const inner = mask.slice(2)
      const broad = broadness(inner, translate)
      if (broad) warnings.push(broad)
      return {
        mask,
        expanded,
        extban: 'mute',
        warnings,
        scope: translate('collectionsUi.channel.mask.mute', {
          parts: describeParts(inner, translate),
        }),
      }
    }
    return {
      mask,
      expanded,
      extban: 'other',
      warnings,
      scope: translate(
        x.negated
          ? 'collectionsUi.channel.mask.extendedNegated'
          : 'collectionsUi.channel.mask.extended',
      ),
    }
  }
  if (!extbans && v.includes('$'))
    return {
      mask,
      expanded,
      warnings: [translate('collectionsUi.channel.mask.unknownExtban')],
    }
  if (extbans?.prefix && v.startsWith(extbans.prefix))
    return {
      mask,
      expanded,
      extban: 'other',
      warnings: [translate('collectionsUi.channel.mask.unknownType', { prefix: extbans.prefix })],
    }
  if (extbans && /^[$~]/.test(v))
    warnings.push(translate('collectionsUi.channel.mask.noExtendedType', { prefix: v[0]! }))
  const { mask: plain, forward } = splitForward(mask, extbans)
  const broad = broadness(plain, translate)
  if (broad) warnings.push(broad)
  return {
    mask,
    expanded,
    warnings,
    scope: translate(
      forward ? 'collectionsUi.channel.mask.matchesForward' : 'collectionsUi.channel.mask.matches',
      { parts: describeParts(plain, translate), channel: forward },
    ),
  }
}

/** `nick!user@host` in words: which of the three parts the mask actually pins down. */
function describeParts(mask: string, translate: ChannelModeTranslator = english) {
  const [nick = '*', rest = '*@*'] = mask.split('!')
  const [user = '*', host = '*'] = rest.split('@')
  const parts: string[] = []
  if (!onlyWildcards(nick))
    parts.push(
      translate(
        nick.includes('*') || nick.includes('?')
          ? 'collectionsUi.channel.mask.nickPattern'
          : 'collectionsUi.channel.mask.nick',
        { value: nick },
      ),
    )
  if (!onlyWildcards(user))
    parts.push(
      translate(
        user.includes('*') || user.includes('?')
          ? 'collectionsUi.channel.mask.usernamePattern'
          : 'collectionsUi.channel.mask.username',
        { value: user },
      ),
    )
  if (!onlyWildcards(host))
    parts.push(
      translate(
        host.includes('*') || host.includes('?')
          ? 'collectionsUi.channel.mask.hostPattern'
          : 'collectionsUi.channel.mask.host',
        { value: host },
      ),
    )
  const joined =
    parts.length === 2
      ? translate('collectionsUi.channel.mask.joinTwo', {
          first: parts[0]!,
          second: parts[1]!,
        })
      : parts.length > 2
        ? translate('collectionsUi.channel.mask.joinMany', {
            first: parts.slice(0, -1).join(', '),
            last: parts.at(-1)!,
          })
        : parts[0]
  return joined
    ? translate('collectionsUi.channel.mask.anyoneWith', { parts: joined })
    : translate('collectionsUi.channel.mask.everyone')
}
