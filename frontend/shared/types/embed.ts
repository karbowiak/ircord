/** Data-only declarations are extracted from provider SFCs without executing their scripts. */
export type EmbedLayout = { ratio: [number, number] } | { height: number }
export type EmbedAttachmentKind = 'image' | 'video' | 'audio' | 'file' | 'gallery'
export interface EmbedImageDimensions {
  width: number
  height: number
}
export interface EmbedMatch {
  /** Exact hosts or *.example.com (the latter also includes the bare host). */
  hosts?: string[]
  /** Whole paths; * matches one segment, ** matches the remaining segments. */
  paths?: string[]
  /** Required, nonempty query parameters, for example YouTube's v. */
  query?: string[]
  extensions?: string[]
}
export interface EmbedDeclaration {
  id: string
  priority: number
  matches?: EmbedMatch[]
  attachments?: EmbedAttachmentKind[]
  attachmentTypes?: string[]
  fallback?: boolean
  layout: EmbedLayout
  /** Initial validated image dimensions fit the reserved box to the height cap and chat width. */
  imageSizing?: boolean
  /** Only the existing authenticated, message-scoped Go resolver can perform extraction. */
  resolver: 'message' | 'none'
  accepts: ('media' | 'file' | 'card' | 'widget')[]
  /** A response cannot select a different adapter. Players must explicitly name their source. */
  providers?: string[]
  /** Backend-approved iframe sources that belong to this adapter. This grants no Go capability. */
  frames?: EmbedMatch[]
  hideUrl?: boolean
  variants?: { matches: EmbedMatch[]; layout: EmbedLayout }[]
}
export interface EmbedProvider extends EmbedDeclaration {
  /** A build-generated lazy import key, never an external URL. */
  component: string
}
export interface EmbedSelection {
  provider: EmbedProvider
  layout: EmbedLayout
  valid: boolean
}
