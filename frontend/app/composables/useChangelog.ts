import { parseChangelog, type Changelog } from '../../shared/utils/changelog'

/**
 * Loads the release notes of the build the server runs now. Each caller (the update dialog, the
 * About settings) keeps its own state; a failure only shows in that view and never touches chat
 * or reloading.
 */
export function useChangelog() {
  const config = useRuntimeConfig()
  /** The build and release this page was loaded with. */
  const running = { buildId: config.app.buildId, version: useAppConfig().release.version }
  const url = `${config.public.appApiBase}/changelog`
  const data = ref<Changelog | null>(null)
  const status = ref<'idle' | 'loading' | 'ready' | 'failed'>('idle')
  let request = 0

  async function load() {
    const id = ++request
    status.value = 'loading'
    try {
      const raw = await $fetch<unknown>(url, { cache: 'no-store', query: { t: Date.now() } })
      if (id !== request) return
      data.value = parseChangelog(raw)
      status.value = data.value ? 'ready' : 'failed'
    } catch {
      if (id !== request) return
      status.value = 'failed'
    }
  }

  return { running, data, status, load }
}
