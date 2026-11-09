/** Public, account-independent metadata also works without client JavaScript. */
export default defineNitroPlugin((nitroApp) => {
  nitroApp.hooks.hook('render:html', (html, { event }) => {
    // Pin the public origin behind TLS proxies with NUXT_SITE_URL. Otherwise use
    // the externally forwarded host/protocol; never include paths or query data.
    const configured = useRuntimeConfig(event).siteUrl
    const request = getRequestURL(event, { xForwardedHost: true, xForwardedProto: true })
    let origin: string
    try {
      const url = new URL(configured || request.origin)
      if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return
      origin = url.origin
    } catch {
      return
    }
    const image = new URL('/icons/icon-512.png', origin).href
    const escape = (value: string) =>
      value.replace(
        /[&<>"']/g,
        (character) =>
          ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]!,
      )
    html.head.push(
      `<meta property="og:image" content="${escape(image)}">`,
      `<meta name="twitter:image" content="${escape(image)}">`,
    )
  })
})
