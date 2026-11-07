/** A channel offered by the composer's “#” autocomplete. */
export interface ChannelSuggestion {
  name: string
  joined: boolean
  users?: number
  topic?: string
  /** Mentions, or else unread messages, for joined channels. */
  unread?: number
}
