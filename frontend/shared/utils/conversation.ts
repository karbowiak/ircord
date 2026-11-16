import type { components } from '../generated/api'
import type { Conversation } from '../types/chat'

/** Fields that change with nearly every event in a busy conversation. */
const activityFields = new Set<string>(['unreadCount', 'mentionCount', 'unreadFrom', 'memberIds'])

const plainObject = (value: unknown): value is Record<string, unknown> =>
  !!value && typeof value === 'object' && !Array.isArray(value)

function sameEntries(a: Record<string, unknown>, b: Record<string, unknown>) {
  const keys = Object.keys(a)
  return keys.length === Object.keys(b).length && keys.every((key) => a[key] === b[key])
}

/**
 * Whether two copies of a conversation differ only in their unread state and member list.
 * Nested settings (member modes, channel settings) compare one level deep, since every server
 * update brings new copies of them.
 */
export function sameConversationSettings(a: Conversation, b: Conversation) {
  if (a === b) return true
  const keys = new Set([...Object.keys(a), ...Object.keys(b)])
  for (const key of keys) {
    if (activityFields.has(key)) continue
    const x = (a as Record<string, unknown>)[key]
    const y = (b as Record<string, unknown>)[key]
    if (x === y) continue
    if (!plainObject(x) || !plainObject(y) || !sameEntries(x, y)) return false
  }
  return true
}

/** The mock and cached UI can omit defaults; emitted wire DTOs cannot. */
export function wireConversation(value: Conversation): components['schemas']['Conversation'] {
  return {
    ...value,
    joined: value.joined ?? true,
    modes: value.modes ?? '',
    memberModes: value.memberModes ?? {},
  }
}
