import type { Conversation, Network } from '../../../shared/types/chat'

/** The channel a settings tab edits: its name, current modes, and MODE/TOPIC commands to it. */
export function useChannelContext(c: () => Conversation, network: () => Network | undefined) {
  const chat = useChatStore()
  return {
    name: computed(() => `#${c().name}`),
    set: computed(() => new Set(c().modes ?? '')),
    /** False when the command couldn't be sent (disconnected). */
    send: (line: string) => chat.raw(line, network()?.id),
  }
}

/** When a topic, mode or list entry was set, for the channel settings tabs. */
export const channelDate = (iso?: string) =>
  iso ? new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' }) : ''
