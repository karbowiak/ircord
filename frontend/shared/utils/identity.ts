import type { Member } from '../types/chat'
import { isHex, readableOn } from './color'

/** Nickname the backend gives an account before it has joined any network. */
const placeholderNick = 'ircord'

/**
 * The name the user explicitly chose for their IRCord profile, or '' when none is set and
 * the network real name or nickname shows instead. Profiles stored before name sources
 * existed count as explicit unless they hold the automatic “You” or the nickname.
 */
export function explicitDisplayName(member: Member | undefined) {
  if (!member?.displayName) return ''
  if (member.nameSource) return member.nameSource === 'profile' ? member.displayName : ''
  return member.displayName === 'You' || member.displayName === member.nick
    ? ''
    : member.displayName
}

/** The name shown while no profile name is set: the network real name, else the nickname. */
export function fallbackDisplayName(member: Member | undefined, nick?: string, realname?: string) {
  if (realname?.trim() && realname !== 'IRCord') return realname
  // Presentation only, until a network gives the account a nickname.
  if (!nick && member?.nick === placeholderNick) return 'You'
  return nick || member?.nick || ''
}

/** Self identity follows the active network; an explicit profile name still wins. */
export function selfForNetwork(
  member: Member | undefined,
  nick?: string,
  realname?: string,
): Member | undefined {
  if (!member) return undefined
  return {
    ...member,
    nick: nick || member.nick,
    displayName: explicitDisplayName(member) || fallbackDisplayName(member, nick, realname),
  }
}

const palettes = {
  dark: ['#e7a1bc', '#98b8f0', '#c1a4ec', '#e4b66c', '#95ceb8', '#e8ad89', '#91cbd1', '#c6c995'],
  light: ['#984263', '#3e649d', '#7852a0', '#816021', '#34705a', '#945c36', '#307177', '#676c27'],
}
const fallbackBackground = { dark: '#2b2b33', light: '#e8e5ef' }

/**
 * Stable per-identity name colour, or the member's own override, adjusted to read clearly on
 * `background` (the active scheme's message background).
 */
export function memberNameColor(
  member: Pick<Member, 'id' | 'account' | 'nameColor'> | undefined,
  theme: 'dark' | 'light' = 'dark',
  background = fallbackBackground[theme],
) {
  if (!member) return theme === 'dark' ? '#ceced7' : '#484351'
  if (isHex(member.nameColor)) return readableOn(member.nameColor, background)
  let hash = 2166136261
  for (const char of `${member.id}:${member.account}`)
    hash = Math.imul(hash ^ char.charCodeAt(0), 16777619)
  hash = Math.imul(hash ^ (hash >>> 16), 0x85ebca6b)
  hash = Math.imul(hash ^ (hash >>> 13), 0xc2b2ae35)
  hash ^= hash >>> 16
  return readableOn(palettes[theme][(hash >>> 0) % palettes[theme].length]!, background)
}

const escape = (value: string) => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
/** Whether text addresses `nick`, with or without “@”, as a whole word (IRC highlight rules). */
export function mentionsNick(text: string, nick: string | undefined) {
  if (!nick) return false
  return new RegExp(`(^|[^\\w-])@?${escape(nick)}(?![\\w-])`, 'i').test(text)
}
