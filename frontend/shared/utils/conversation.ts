import type { components } from '../generated/api'
import type { Conversation } from '../types/chat'

/** The mock and cached UI can omit defaults; emitted wire DTOs cannot. */
export function wireConversation(value: Conversation): components['schemas']['Conversation'] {
  return { ...value, joined: value.joined ?? true, modes: value.modes ?? '', memberModes: value.memberModes ?? {} }
}
