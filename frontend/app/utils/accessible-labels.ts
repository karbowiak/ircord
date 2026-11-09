// Screen-reader text for states the interface shows only as dots, badges, colour or opacity.
import type { Member } from '../../shared/types/chat'

/** “1 mention”, “3 mentions”: a count with its noun in the right number. */
export function countLabel(count: number, one: string, other: string) {
  return `${count} ${count === 1 ? one : other}`
}

/** What a presence dot shows, or nothing for someone online on a computer. */
export function presenceLabel(member: Pick<Member, 'status' | 'device'> | undefined) {
  if (!member || member.status === 'offline') return 'offline'
  const device =
    member.device === 'phone' ? 'on a phone' : member.device === 'tablet' ? 'on a tablet' : ''
  if (member.status === 'away') return device ? `away, ${device}` : 'away'
  return device
}
