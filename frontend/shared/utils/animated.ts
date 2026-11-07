// Whether an image file is animated, read from its first bytes. Only same-origin files (uploads)
// can be read; linked images from other sites are classified by URL in links.ts.

const ascii = (bytes: Uint8Array, at: number, text: string) =>
  [...text].every((c, i) => bytes[at + i] === c.charCodeAt(0))
const u32 = (bytes: Uint8Array, at: number) =>
  ((bytes[at]! << 24) | (bytes[at + 1]! << 16) | (bytes[at + 2]! << 8) | bytes[at + 3]!) >>> 0

/**
 * True for GIF (labelled as GIF even with one frame, like a .gif link), APNG, animated WebP and
 * AVIF image sequences. Unknown or truncated data counts as still.
 */
export function isAnimatedImage(bytes: Uint8Array): boolean {
  if (ascii(bytes, 0, 'GIF8')) return true
  // PNG: an acTL chunk before the first IDAT makes it an APNG.
  if (bytes[0] === 0x89 && ascii(bytes, 1, 'PNG')) {
    for (let at = 8; at + 8 <= bytes.length;) {
      if (ascii(bytes, at + 4, 'acTL')) return true
      if (ascii(bytes, at + 4, 'IDAT')) return false
      at += 12 + u32(bytes, at)
    }
    return false
  }
  // WebP: the extended header's animation flag.
  if (ascii(bytes, 0, 'RIFF') && ascii(bytes, 8, 'WEBP'))
    return ascii(bytes, 12, 'VP8X') && ((bytes[20] ?? 0) & 0x02) !== 0
  // AVIF: an image sequence declares the `avis` brand in its ftyp box.
  if (ascii(bytes, 4, 'ftyp')) {
    const end = Math.min(u32(bytes, 0), bytes.length)
    for (let at = 8; at + 4 <= end; at += 4) if (ascii(bytes, at, 'avis')) return true
  }
  return false
}

const sniffBytes = 64 * 1024
const cache = new Map<string, Promise<boolean>>()

/** Reads the start of a same-origin image to tell whether it is animated. */
export function sniffAnimated(url: string): Promise<boolean> {
  let found = cache.get(url)
  if (!found) {
    found = (async () => {
      const response = await fetch(url, {
        credentials: 'include',
        headers: { Range: `bytes=0-${sniffBytes - 1}` },
      })
      const reader = response.body?.getReader()
      if (!response.ok || !reader) return false
      const chunks: Uint8Array[] = []
      let size = 0
      while (size < sniffBytes) {
        const { done, value } = await reader.read()
        if (done) break
        chunks.push(value)
        size += value.length
      }
      void reader.cancel().catch(() => {})
      const bytes = new Uint8Array(size)
      let at = 0
      for (const chunk of chunks) {
        bytes.set(chunk, at)
        at += chunk.length
      }
      return isAnimatedImage(bytes)
    })().catch(() => false)
    cache.set(url, found)
  }
  return found
}
