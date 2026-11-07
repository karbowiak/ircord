import { createHash } from 'node:crypto'

/**
 * Content-Security-Policy for the app page. Inline scripts (the appearance boot script and
 * Nuxt's config payload) are allowed by hash, so injected markup cannot run script. Styles allow
 * inline because themes and Vue style bindings set them. Development is skipped: Vite's HMR needs
 * looser rules.
 */
export default defineNitroPlugin((nitroApp) => {
  if (import.meta.dev) return
  const config = useRuntimeConfig().public
  const origin = (value: string) => {
    try {
      return new URL(value).origin
    } catch {
      return ''
    }
  }
  const apiOrigin = origin(config.apiBase)
  const appOrigin = origin(config.appApiBase)
  const socket = config.websocketUrl
    ? origin(config.websocketUrl)
    : (apiOrigin || origin(useRuntimeConfig().siteUrl)).replace(/^http/, 'ws')
  const connections = [...new Set([apiOrigin, appOrigin, socket].filter(Boolean))].join(' ')
  nitroApp.hooks.hook('render:response', (response) => {
    if (typeof response.body !== 'string') return
    const hashes = [...response.body.matchAll(/<script(?![^>]*\bsrc=)[^>]*>([\s\S]*?)<\/script>/g)]
      .map((match) => match[1]!)
      .filter(Boolean)
      .map((code) => `'sha256-${createHash('sha256').update(code).digest('base64')}'`)
    const policy = [
      "default-src 'self'",
      `script-src 'self' ${hashes.join(' ')}`,
      "style-src 'self' 'unsafe-inline'",
      // Chat shows images from anywhere (link previews, avatars, GIF and emote CDNs).
      `img-src 'self' data: blob: https: ${apiOrigin}`.trim(),
      `media-src 'self' blob: https: ${apiOrigin}`.trim(),
      "font-src 'self' data:",
      `connect-src 'self' https://api.klipy.com ${connections}`.trim(),
      "worker-src 'self'",
      "manifest-src 'self'",
      "object-src 'none'",
      "base-uri 'self'",
      "form-action 'self'",
      "frame-ancestors 'none'",
    ].join('; ')
    response.headers = { ...response.headers, 'Content-Security-Policy': policy }
  })
})
