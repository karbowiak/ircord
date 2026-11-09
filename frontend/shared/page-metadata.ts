/** Public copy shared by Nitro and the client, ready for later catalog extraction.
 * Never derive it from accounts, route parameters, query strings or fragments. */
export const publicPages = {
  '/': { title: 'IRCord', description: 'Your IRC networks, channels and messages in one place.' },
  '/channel': { title: 'Channels', description: 'Read and join your IRC channels with IRCord.' },
  '/dm': { title: 'Direct messages', description: 'Keep up with your direct messages in IRCord.' },
  '/server': { title: 'Server', description: 'Manage your IRC connections with IRCord.' },
  '/search': { title: 'Search', description: 'Find messages in your IRCord conversations.' },
  '/inbox': { title: 'Inbox', description: 'Catch up on mentions and direct messages in IRCord.' },
  '/saved': { title: 'Saved', description: 'Return to messages you saved in IRCord.' },
  '/drafts': { title: 'Drafts', description: 'Continue messages you started in IRCord.' },
  '/links': { title: 'Links', description: 'Browse links shared in your IRCord conversations.' },
} as const

/** Only fixed, public route labels may enter canonical and Open Graph URLs. */
export function publicPagePath(path: string): keyof typeof publicPages {
  const pathname = path.split(/[?#]/, 1)[0]!.replace(/\/$/, '') || '/'
  if (Object.hasOwn(publicPages, pathname)) return pathname as keyof typeof publicPages
  const conversation = /^\/(channel|dm|server)\/[^/]+$/.exec(pathname)
  return conversation ? (`/${conversation[1]}` as '/channel' | '/dm' | '/server') : '/'
}

export function publicOrigin(value: string): string | undefined {
  try {
    const url = new URL(value)
    if (['http:', 'https:'].includes(url.protocol) && !url.username && !url.password)
      return url.origin
  } catch {
    // Invalid configuration or forwarded input must not become markup or a URL.
  }
}

export function publicPageHead(
  path: string,
  origin?: string,
  translate?: (key: string, values?: Record<string, string>) => string,
) {
  const publicPath = publicPagePath(path)
  const fallback = publicPages[publicPath]
  const key = publicPath === '/' ? 'home' : publicPath.slice(1)
  const page = translate
    ? {
        title: translate(`workspaceUi.metadata.${key}.title`),
        description: translate(`workspaceUi.metadata.${key}.description`),
      }
    : fallback
  const title =
    publicPath === '/'
      ? page.title
      : translate
        ? translate('workspaceUi.metadata.pageTitle', { title: page.title })
        : `${page.title} · IRCord`
  const logo = translate ? translate('workspaceUi.metadata.logo') : 'IRCord logo'
  const url = origin && publicOrigin(origin)
  const image = url ? new URL('/icons/icon-512.png', url).href : undefined
  const canonical = url ? new URL(publicPath, url).href : undefined
  return {
    title,
    meta: [
      { name: 'description', content: page.description },
      { property: 'og:title', content: title },
      { property: 'og:description', content: page.description },
      { property: 'og:site_name', content: 'IRCord' },
      { property: 'og:type', content: 'website' },
      { property: 'og:image:width', content: '512' },
      { property: 'og:image:height', content: '512' },
      { property: 'og:image:alt', content: logo },
      { name: 'twitter:card', content: 'summary' },
      { name: 'twitter:title', content: title },
      { name: 'twitter:description', content: page.description },
      { name: 'twitter:image:alt', content: logo },
      ...(image
        ? [
            { property: 'og:image', content: image },
            { name: 'twitter:image', content: image },
          ]
        : []),
      ...(canonical ? [{ property: 'og:url', content: canonical }] : []),
    ],
    link: canonical ? [{ key: 'public-canonical', rel: 'canonical', href: canonical }] : [],
  }
}
