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
  shortcuts: () => {},
  exportLog: (_conversationId: string) => {},
  jumpToDate: (_conversationId: string) => {},
  reply: (_message: Message) => {},
}
export function useWorkspaceActions() {
  return actions
}
