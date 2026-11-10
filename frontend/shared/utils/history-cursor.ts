/** A clipped browser window must page from its own first retained message, not an old page. */
export const messageBoundary = (id: string) => `message:${encodeURIComponent(id)}`

/** Client cursors contain an identity only. The server supplies its own opaque page cursor. */
export function boundaryMessage(cursor?: string): string | undefined {
  if (!cursor || cursor.length > 4096) return undefined
  try {
    if (cursor.startsWith('message:')) return decodeURIComponent(cursor.slice(8)) || undefined
    if (cursor.startsWith('offline:')) {
      const raw = cursor.slice(8).replace(/-/g, '+').replace(/_/g, '/')
      const value = JSON.parse(
        new TextDecoder().decode(Uint8Array.from(atob(raw), (c) => c.charCodeAt(0))),
      )
      return typeof value.id === 'string' && value.id ? value.id : undefined
    }
  } catch {
    // A malformed local cursor must never be mistaken for the end of history.
  }
  return undefined
}
