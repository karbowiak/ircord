import type { components } from '../../shared/generated/api'
import type { EmbedSelection } from '../../shared/types/embed'
import type { Attachment } from '../../shared/types/chat'

/** One owner calls useEmbed; renderers receive this read-only snapshot plus explicit callbacks. */
export interface EmbedContext {
  url: string
  host: string
  selection: EmbedSelection
  surface: 'timeline' | 'catalog'
  content?: components['schemas']['LinkMediaResponse']
  assets: components['schemas']['LinkMediaAsset'][]
  widget?: components['schemas']['LinkMediaWidget']
  file?: components['schemas']['LinkMediaFile']
  card: components['schemas']['LinkMediaCard']
  title: string
  attachment?: Attachment
  downloadSource?: string
  nearby: boolean
  pending: boolean
  expired: boolean
  offline: boolean
  failed: boolean
  widgetKey: string
  ready: (value: boolean) => void
  unavailable: () => void
  viewing: (value: boolean) => void
}
