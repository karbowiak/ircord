import type { Message, NetworkInput } from '../../../shared/types/chat'

/**
 * Overlays and panels the workspace owns, callable from menus, commands and deep components
 * without threading events through every layer. AppWorkspace fills these in.
 */
const actions = {
  profile: (_id: string) => {},
  settings: (_tab?: string) => {},
  editNetwork: (_id?: string) => {},
  /** The add-network dialog, optionally prefilled (irc:// links). */
  addNetwork: (_prefill?: Partial<NetworkInput>) => {},
  channelSettings: (_conversationId: string, _tab?: string) => {},
  party: (_mode: 'create' | 'details', _conversationId?: string) => {},
  /** Opens the channel directory, optionally filtered by name. */
  join: (_filter?: string) => {},
  createChannel: () => {},
  /** The Search page, optionally searching messages for `query`. */
  search: (_query?: string) => {},
  /** The overview of unsent text across conversations on this device. */
  drafts: () => {},
  /** Your private saved messages. */
  saved: () => {},
  /** Links posted in a channel (by id) or, without one, in all your channels. */
  links: (_conversationId?: string) => {},
  /** Your private inbox of mentions, replies and direct messages. */
  inbox: () => {},
  /** Reveal a message from a workspace page in its conversation. */
  reveal: async (_conversationId: string, _messageId: string): Promise<void> => {},
  resumeDraft: async (_conversationId: string): Promise<void> => {},
  shortcuts: () => {},
  /** Server owners' Administration panel (#165). */
  administration: () => {},
  /** Toggles the composer's GIF picker; false when there is none to open (Ctrl/⌘ G). */
  gifs: (): boolean => false,
  /** Opens the inline status editor for the selected network, if a composer is mounted. */
  status: (_networkId?: string): boolean => false,
  /** Puts the caret in the message box; false when there is none to focus. */
  focusComposer: (): boolean => false,
  /** Scrolls the timeline to the “New messages” line; false when there is none. */
  jumpToUnread: (): boolean => false,
  /** Scrolls the timeline to the newest message; false without a timeline. */
  jumpToPresent: (): boolean => false,
  exportLog: (_conversationId: string) => {},
  jumpToDate: (_conversationId: string) => {},
  reply: (_message: Message) => {},
}
export function useWorkspaceActions() {
  return actions
}
