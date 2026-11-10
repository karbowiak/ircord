import type { Conversation } from '../types/chat'

export const workspacePages = {
  '/search': { title: 'Search', icon: 'search' },
  '/inbox': { title: 'Inbox', icon: 'inbox' },
  '/saved': { title: 'Saved', icon: 'bookmark' },
  '/drafts': { title: 'Drafts', icon: 'pencil' },
  '/links': { title: 'Links', icon: 'link' },
} as const

export function isConversationPath(path: string) {
  return path === '/' || /^\/(channel|dm|server)(\/|$)/.test(path)
}
export function conversationPath(id: string, conversations: readonly Conversation[]) {
  if (id.startsWith('server:')) return `/server/${encodeURIComponent(id.slice(7))}`
  const conversation = conversations.find((c) => c.id === id)
  return conversation
    ? `/${conversation.kind === 'channel' && !conversation.party ? 'channel' : 'dm'}/${encodeURIComponent(id)}`
    : null
}
