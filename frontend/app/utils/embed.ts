import { embedProviders } from '#embed-manifest'
import type { CSSProperties } from 'vue'
import type {
  EmbedAttachmentKind,
  EmbedImageDimensions,
  EmbedMatch,
  EmbedProvider,
  EmbedSelection,
} from '../../shared/types/embed'

const hosts = new Map<string, Set<EmbedProvider>>()
const domains = new Map<string, Set<EmbedProvider>>()
const extensions = new Map<string, Set<EmbedProvider>>()
const attachments = new Map<EmbedAttachmentKind, EmbedProvider[]>()
const attachmentTypes = new Map<string, EmbedProvider[]>()
function add(index: Map<string, Set<EmbedProvider>>, key: string, provider: EmbedProvider) {
  if (!index.has(key)) index.set(key, new Set())
  index.get(key)!.add(provider)
}
for (const provider of embedProviders) {
  for (const match of provider.matches ?? []) {
    for (const host of match.hosts ?? [])
      add(host.startsWith('*.') ? domains : hosts, host.replace(/^\*\./, ''), provider)
    for (const extension of match.extensions ?? []) add(extensions, extension, provider)
  }
  for (const kind of provider.attachments ?? []) {
    if (!attachments.has(kind)) attachments.set(kind, [])
    attachments.get(kind)!.push(provider)
  }
  for (const type of provider.attachmentTypes ?? []) {
    if (!attachmentTypes.has(type)) attachmentTypes.set(type, [])
    attachmentTypes.get(type)!.push(provider)
  }
}
const fallback = embedProviders.find((provider) => provider.fallback)!
const priority = (a: EmbedProvider, b: EmbedProvider) =>
  b.priority - a.priority || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0)
function pathMatches(pattern: string, path: string) {
  const expected = pattern.replace(/\/$/, '').split('/')
  const actual = path.replace(/\/$/, '').split('/')
  return (
    expected.every(
      (part, i) => part === '**' || (i < actual.length && (part === '*' || part === actual[i])),
    ) &&
    (expected.at(-1) === '**' || expected.length === actual.length)
  )
}
export function embedMatches(rule: EmbedMatch, url: URL) {
  const host = url.hostname.toLowerCase().replace(/\.$/, '')
  return (
    (!rule.hosts ||
      rule.hosts.some((entry) =>
        entry.startsWith('*.')
          ? host === entry.slice(2) || host.endsWith(entry.slice(1))
          : host === entry,
      )) &&
    (!rule.paths || rule.paths.some((path) => pathMatches(path, url.pathname))) &&
    (!rule.query || rule.query.every((key) => !!url.searchParams.get(key))) &&
    (!rule.extensions ||
      rule.extensions.includes(url.pathname.split('.').at(-1)?.toLowerCase() ?? ''))
  )
}
/** Selection and geometry depend only on the posted URL or trusted upload metadata. */
export function selectEmbed(
  raw: string,
  attachmentKind?: EmbedAttachmentKind,
  contentType?: string,
): EmbedSelection {
  let url: URL
  try {
    if (raw.length > 8192 || raw !== raw.trim()) throw new Error('Invalid URL')
    url = new URL(raw)
    if (!/^https?:$/.test(url.protocol) || url.username || url.password)
      throw new Error('Invalid URL')
  } catch {
    return { provider: fallback, layout: fallback.layout, valid: false }
  }
  let provider: EmbedProvider | undefined
  if (attachmentKind)
    provider =
      attachmentTypes.get(contentType?.split(';')[0]?.trim().toLowerCase() ?? '')?.[0] ??
      attachments.get(attachmentKind)?.[0]
  if (!provider) {
    const candidates = new Set<EmbedProvider>()
    const include = (values?: Set<EmbedProvider>) =>
      values?.forEach((value) => candidates.add(value))
    const host = url.hostname.toLowerCase().replace(/\.$/, '')
    include(hosts.get(host))
    const labels = host.split('.')
    for (let i = 0; i < labels.length - 1; i++) include(domains.get(labels.slice(i).join('.')))
    include(extensions.get(url.pathname.split('.').at(-1)?.toLowerCase() ?? ''))
    provider = [...candidates]
      .sort(priority)
      .find((candidate) => candidate.matches?.some((rule) => embedMatches(rule, url)))
  }
  provider ??= fallback
  const variant = provider.variants?.find((entry) =>
    entry.matches.some((rule) => embedMatches(rule, url)),
  )
  return { provider, layout: variant?.layout ?? provider.layout, valid: true }
}
export function embedStyle(
  selection: EmbedSelection,
  surface: 'timeline' | 'catalog' = 'timeline',
  image?: EmbedImageDimensions,
): CSSProperties {
  const layout = surface === 'catalog' ? { ratio: [16, 9] } : selection.layout
  const fitted =
    surface === 'timeline' && selection.provider.imageSizing && image && 'height' in layout
      ? {
          width: `min(100%, ${(layout.height * image.width) / image.height}px)`,
          aspectRatio: `${image.width} / ${image.height}`,
        }
      : undefined
  return {
    position: 'relative',
    ...embedContainerStyle(selection, surface),
    minWidth: '0',
    overflow: 'hidden',
    '--embed-height': '100%',
    ...(fitted ??
      ('ratio' in layout
        ? { aspectRatio: layout.ratio.join(' / ') }
        : { height: `${layout.height}px` })),
  }
}
export function embedContainerStyle(
  selection: EmbedSelection,
  surface: 'timeline' | 'catalog' = 'timeline',
): CSSProperties {
  return {
    width: '100%',
    maxWidth: surface === 'catalog' || selection.provider.imageSizing ? '100%' : '480px',
  }
}

/** Size data is never a URL or render capability; reject invalid arithmetic before CSS. */
export function embedImageDimensions(value?: {
  width?: number
  height?: number
}): EmbedImageDimensions | undefined {
  if (
    !value ||
    !Number.isSafeInteger(value.width) ||
    !Number.isSafeInteger(value.height) ||
    value.width! <= 0 ||
    value.height! <= 0 ||
    value.width! > 100_000 ||
    value.height! > 100_000
  )
    return undefined
  return { width: value.width!, height: value.height! }
}
