/**
 * Default avatars shipped with the client (public/avatars/defaults). The set is listed in
 * manifest.json beside the images, so adding an entry there makes it selectable without code
 * changes. Entries are never renumbered: a chosen default is saved as its URL in the profile.
 */
export const DEFAULT_AVATAR_PATH = '/avatars/defaults/'
export const DEFAULT_AVATAR_MANIFEST = `${DEFAULT_AVATAR_PATH}manifest.json`

export interface DefaultAvatar {
  id: string
  file: string
  category: string
}

const FILE = /^[\w-]+\.(?:png|jpe?g|webp)$/i
const ID = /^[\w-]{1,64}$/

/** The usable entries of a manifest, in manifest order; malformed or repeated ones are skipped. */
export function parseAvatarManifest(value: unknown): DefaultAvatar[] {
  const list = (value as { avatars?: unknown } | null)?.avatars
  if (!Array.isArray(list)) return []
  const seen = new Set<string>()
  const avatars: DefaultAvatar[] = []
  for (const entry of list) {
    const { id, file, category } = (entry ?? {}) as Record<string, unknown>
    if (typeof id !== 'string' || !ID.test(id) || seen.has(id)) continue
    if (typeof file !== 'string' || !FILE.test(file)) continue
    seen.add(id)
    avatars.push({ id, file, category: typeof category === 'string' ? category : '' })
  }
  return avatars
}

/** 32-bit FNV-1a over the UTF-8 bytes of `text`. */
export function fnv1a(text: string): number {
  let hash = 0x811c9dc5
  for (const byte of new TextEncoder().encode(text)) {
    hash ^= byte
    hash = Math.imul(hash, 0x01000193) >>> 0
  }
  return hash >>> 0
}

/** The avatar suggested for a username: the same name (any case) always gets the same one. */
export function suggestAvatar(
  username: string,
  avatars: readonly DefaultAvatar[],
): DefaultAvatar | undefined {
  const name = username.trim().toLowerCase()
  if (!name || !avatars.length) return undefined
  return avatars[fnv1a(name) % avatars.length]
}

/** The absolute URL saved in a profile, on this instance's own origin. */
export function defaultAvatarUrl(avatar: DefaultAvatar, origin: string): string {
  return new URL(DEFAULT_AVATAR_PATH + avatar.file, origin).href
}

/** The default an avatar URL points at on this origin, if it is one. */
export function defaultAvatarFor(
  url: string | null | undefined,
  avatars: readonly DefaultAvatar[],
  origin: string,
): DefaultAvatar | undefined {
  if (!url) return undefined
  try {
    const parsed = new URL(url, origin)
    if (parsed.origin !== new URL(origin).origin) return undefined
    if (!parsed.pathname.startsWith(DEFAULT_AVATAR_PATH)) return undefined
    const file = parsed.pathname.slice(DEFAULT_AVATAR_PATH.length)
    return avatars.find((avatar) => avatar.file === file)
  } catch {
    return undefined
  }
}
