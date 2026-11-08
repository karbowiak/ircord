import type { Changelog } from '../../../shared/utils/changelog'

/**
 * Release notes for the build this server runs, baked into the image at release time
 * (scripts/release-changelog.mjs). Public: they are the repository's published releases. The
 * build id lets a client match the notes to the build the manifest announced.
 */
export default defineEventHandler(async (event): Promise<Changelog> => {
  setHeader(event, 'Cache-Control', 'no-cache')
  const data = await useStorage('assets:server')
    .getItem<{ releases?: Changelog['releases'] }>('changelog.json')
    .catch(() => null)
  return {
    buildId: useRuntimeConfig(event).app.buildId,
    version: useAppConfig().release.version,
    releases: Array.isArray(data?.releases) ? data.releases : [],
  }
})
