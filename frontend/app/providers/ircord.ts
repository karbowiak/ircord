import { defineProvider } from '@nuxt/image/runtime'

/**
 * Keep the backend-issued URL intact, including upload tokens and signed query
 * strings. Go owns access, caching, retention and deletion. The current CDN has
 * no resize contract; modifiers deliberately do not change its original URL.
 */
export default defineProvider({ getImage: (url) => ({ url }) })
