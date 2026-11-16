/**
 * Profile pictures load only from this IRCord instance (#278). Anyone on IRC can set their avatar
 * to an address they control, and every viewer who loads it reveals their IP address and when
 * they are online. Pictures on another host show the person's initial instead.
 *
 * `origins` lists the instance's own origins: the page's, and the API's when it is served from
 * another host. A relative path resolves against the first origin. An image picked on this device
 * (a `data:` or `blob:` URL, as in the profile editor's preview) needs no request and loads too.
 */
export function instanceImageUrl(
  value: string | null | undefined,
  origins: readonly string[],
): string | undefined {
  if (!value) return undefined
  if (/^data:image\//i.test(value) || /^blob:/i.test(value)) return value
  if (!origins.length) return undefined
  try {
    const url = new URL(value, origins[0])
    if (url.protocol !== 'https:' && url.protocol !== 'http:') return undefined
    if (url.username || url.password) return undefined
    return origins.some((origin) => sameOrigin(origin, url.origin)) ? value : undefined
  } catch {
    return undefined
  }
}

function sameOrigin(candidate: string, origin: string) {
  try {
    return new URL(candidate).origin === origin
  } catch {
    return false
  }
}
