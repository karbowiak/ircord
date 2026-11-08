import type { Message, NetworkInput } from '../../shared/types/chat'

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
  join: () => {},
  search: () => {},
  /** The overview of unsent text across conversations on this device. */
  drafts: () => {},
  /** Your private saved messages. */
  saved: () => {},
  /** Links posted in a channel (by id) or, without one, in all your channels. */
  links: (_conversationId?: string) => {},
  /** Your private inbox of mentions, replies and direct messages. */
  inbox: () => {},
  shortcuts: () => {},
  /** Server owners' Administration panel (#165). */
  administration: () => {},
  /** Toggles the composer's GIF picker; false when there is none to open (Ctrl/⌘ G). */
  gifs: (): boolean => false,
  exportLog: (_conversationId: string) => {},
  jumpToDate: (_conversationId: string) => {},
  reply: (_message: Message) => {},
}
export function useWorkspaceActions() {
  return actions
}
