// Screen-reader text for states the interface shows only as dots, badges, colour or opacity.
import type { Member } from '../../shared/types/chat'

/** “1 mention”, “3 mentions”: a count with its noun in the right number. */
export function countLabel(count: number, one: string, other: string) {
  return `${count} ${count === 1 ? one : other}`
}

/** What a presence dot shows, or nothing for someone online on a computer. */
export function presenceLabel(
  member: Pick<Member, 'status' | 'device'> | undefined,
  translate?: (key: string) => string,
) {
  const label = (key: string, fallback: string) =>
    translate?.(`workspaceUi.presence.${key}`) ?? fallback
  if (!member || member.status === 'offline') return label('offline', 'offline')
  const device = member.device === 'phone' ? 'phone' : member.device === 'tablet' ? 'tablet' : ''
  if (member.status === 'away')
    return device === 'phone'
      ? label('awayPhone', 'away, on a phone')
      : device === 'tablet'
        ? label('awayTablet', 'away, on a tablet')
        : label('away', 'away')
  if (device === 'phone') return label('phone', 'on a phone')
  if (device === 'tablet') return label('tablet', 'on a tablet')
  return device
}
