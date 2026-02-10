import { isImageUrl, isVideoUrl } from '~/composables/useMediaUrl'

export type MediaResolveResponse = {
  sourceUrl: string
  playbackUrl: string | null
  previewUrl: string | null
  type: 'video' | 'image' | null
  title: string | null
  description: string | null
  siteName: string | null
  canonicalUrl: string | null
}

export type LinkPreviewCard = {
  url: string
  title: string | null
  description: string | null
  siteName: string | null
  previewUrl: string | null
  canonicalUrl: string | null
}

const mediaResolveCache = new Map<string, MediaResolveResponse | null>()
const urlRegex = /(https?:\/\/[^\s]+)/g

function getYouTubeEmbedUrl(url: string): string | null {
  try {
    const parsed = new URL(url)

    if (parsed.hostname.includes('youtu.be')) {
      const id = parsed.pathname.replace('/', '').trim()
      return id ? `https://www.youtube.com/embed/${id}` : null
    }

    if (parsed.hostname.includes('youtube.com')) {
      const id = parsed.searchParams.get('v')
      if (id) return `https://www.youtube.com/embed/${id}`

      if (parsed.pathname.startsWith('/shorts/')) {
        const shortId = parsed.pathname.split('/shorts/')[1]
        return shortId ? `https://www.youtube.com/embed/${shortId}` : null
      }
    }
  } catch {
    return null
  }

  return null
}

function extractUrls(content: string): string[] {
  return Array.from(new Set(content.match(urlRegex) ?? []))
}

export function useMessageEmbeds(content: Ref<string> | ComputedRef<string>) {
  const resolvedPageMediaByUrl = reactive<Record<string, MediaResolveResponse | null>>({})
  const resolvingPageMediaByUrl = reactive<Record<string, boolean>>({})

  const urls = computed(() => extractUrls(unref(content)))

  const mediaPreviewUrls = computed(() => {
    const direct = urls.value.filter((url) => isImageUrl(url) || isVideoUrl(url))
    const resolved = urls.value
      .map((url) => resolvedPageMediaByUrl[url])
      .map((entry) => entry?.playbackUrl || null)
      .filter((value): value is string => Boolean(value))

    return Array.from(new Set([...direct, ...resolved]))
  })

  const youtubeEmbedUrls = computed(() =>
    urls.value
      .map(getYouTubeEmbedUrl)
      .filter((value): value is string => Boolean(value))
  )

  const linkPreviewUrls = computed(() =>
    urls.value.filter((url) => {
      if (isImageUrl(url) || isVideoUrl(url) || getYouTubeEmbedUrl(url)) return false
      if (resolvingPageMediaByUrl[url]) return false

      const resolved = resolvedPageMediaByUrl[url]
      if (!resolved) return true

      return !resolved.playbackUrl
    })
  )

  const linkPreviewCards = computed<LinkPreviewCard[]>(() =>
    linkPreviewUrls.value.map((url) => {
      const resolved = resolvedPageMediaByUrl[url]
      return {
        url,
        title: resolved?.title || null,
        description: resolved?.description || null,
        siteName: resolved?.siteName || null,
        previewUrl: resolved?.previewUrl || null,
        canonicalUrl: resolved?.canonicalUrl || null,
      }
    })
  )

  const resolvingPageUrls = computed(() =>
    urls.value.filter((url) => !isImageUrl(url) && !isVideoUrl(url) && !getYouTubeEmbedUrl(url) && resolvingPageMediaByUrl[url])
  )

  watch(
    urls,
    async (nextUrls) => {
      const pageUrls = nextUrls.filter((url) => !isImageUrl(url) && !isVideoUrl(url) && !getYouTubeEmbedUrl(url))
      if (!pageUrls.length) return

      await Promise.all(pageUrls.map(async (url) => {
        if (mediaResolveCache.has(url)) {
          resolvedPageMediaByUrl[url] = mediaResolveCache.get(url) || null
          resolvingPageMediaByUrl[url] = false
          return
        }

        resolvingPageMediaByUrl[url] = true

        try {
          const response = await $fetch<MediaResolveResponse>('/api/media-resolve', {
            query: { url },
          })

          mediaResolveCache.set(url, response)
          resolvedPageMediaByUrl[url] = response
        } catch {
          mediaResolveCache.set(url, null)
          resolvedPageMediaByUrl[url] = null
        } finally {
          resolvingPageMediaByUrl[url] = false
        }
      }))
    },
    { immediate: true },
  )

  return {
    mediaPreviewUrls,
    youtubeEmbedUrls,
    resolvingPageUrls,
    linkPreviewCards,
  }
}
