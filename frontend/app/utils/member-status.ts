import type { Member } from '../../shared/types/chat'
import { statusShowing } from '../../shared/utils/profile'

/** The readable custom status, independently of IRC presence. */
export function memberStatusLabel(member: Member | undefined, now: number) {
  return member && statusShowing(member, now)
    ? [member.statusEmoji, member.statusText].filter(Boolean).join(' ')
    : ''
}
