import { publicOrigin, publicPageHead } from '../../shared/page-metadata'

const escape = (value: string) =>
  value.replace(
    /[&<>"']/g,
    (character) =>
      ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]!,
  )

/** Preview clients do not execute JavaScript. Keep share metadata account-independent. */
export default defineNitroPlugin((nitroApp) => {
  nitroApp.hooks.hook('render:html', (html, { event }) => {
    // Pin the public origin behind TLS proxies with NUXT_SITE_URL. Otherwise use
    // the externally forwarded host/protocol; never include paths or query data.
    const configured = useRuntimeConfig(event).siteUrl
    const request = getRequestURL(event, { xForwardedHost: true, xForwardedProto: true })
    const origin = publicOrigin(configured) ?? publicOrigin(request.origin)
    const head = publicPageHead(request.pathname, origin)
    // Replace the SPA shell's defaults rather than adding competing public tags.
    html.head = html.head.map((part) =>
      part
        .replace(/<title\b[^>]*>[\s\S]*?<\/title>/gi, '')
        .replace(
          /<meta\b(?=[^>]*(?:name|property)=["'](?:description|og:[^"']+|twitter:[^"']+)["'])[^>]*>/gi,
          '',
        )
        .replace(/<link\b(?=[^>]*rel=["']canonical["'])[^>]*>/gi, ''),
    )
    html.head.push(
      `<title>${escape(head.title)}</title>`,
      ...head.meta.map((tag) =>
        'property' in tag
          ? `<meta property="${tag.property}" content="${escape(tag.content)}">`
          : `<meta name="${tag.name}" content="${escape(tag.content)}">`,
      ),
      ...head.link.map((tag) => `<link rel="canonical" href="${escape(tag.href)}">`),
    )
  })
})
