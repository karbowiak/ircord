/** Where IRCord's source, issues, releases and license live. */
export const PROJECT_URL = 'https://github.com/karbowiak/ircord'

/** A release tag such as v0.1.2 (optionally with a prerelease suffix). */
const RELEASE = /^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/

/**
 * The public facts About shows about the running build: the release tag the image was built from
 * (empty for development and untagged builds) and a short form of Nuxt's build id, which every
 * page already receives. Anything that does not look like one of those is left out rather than
 * shown, so a stray build argument never reaches the page.
 */
export function aboutBuild(version: string | undefined, buildId: string | undefined) {
  const release = RELEASE.test(version ?? '') ? version! : ''
  const id = /^[0-9A-Za-z-]+$/.test(buildId ?? '') ? buildId!.slice(0, 8) : ''
  return { release, id }
}
