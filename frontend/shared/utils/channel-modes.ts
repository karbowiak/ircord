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

/** List modes (type A). */
export const listInfo: Record<string, { label: string; singular: string; description: string }> = {
  b: { label: 'Bans', singular: 'ban', description: 'People matching these masks can’t join.' },
  e: {
    label: 'Ban exceptions',
    singular: 'exception',
    description: 'Masks that can join even when they match a ban.',
  },
  I: {
    label: 'Invite exceptions',
    singular: 'invite exception',
    description: 'Masks that can join an invite-only channel without an invite.',
  },
  q: { label: 'Quiets', singular: 'quiet', description: 'People matching these masks can’t talk.' },
}

/** A mask from a nick (`nick` → `nick!*@*`); masks are kept as typed. */
export const toMask = (value: string) => {
  const v = value.trim()
  return v.includes('!') || v.includes('@') || v.includes('$') ? v : `${v}!*@*`
}
