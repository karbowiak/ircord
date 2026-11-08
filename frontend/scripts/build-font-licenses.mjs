// Collects the license of every self-hosted Fontsource font into one public notice, so the font
// files the app serves travel with their copyright and license text (SIL OFL 1.1 requires it).
// Run after adding or updating a font package: bun scripts/build-font-licenses.mjs
// tests/appearance.test.ts fails while the committed notice differs from what this renders.
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { pathToFileURL } from 'node:url'

const frontend = new URL('../', import.meta.url)
const read = (path) => readFileSync(new URL(path, frontend), 'utf8')
export const noticePath = new URL('public/data/font-licenses.txt', frontend)

/** The @fontsource packages the app depends on, sorted. */
export const fontPackages = () =>
  Object.keys(JSON.parse(read('package.json')).dependencies)
    .filter((name) => name.startsWith('@fontsource'))
    .sort()

/** The notice for the installed font packages: each name, version, license id and license text. */
export function renderFontLicenses() {
  const sections = fontPackages().map((name) => {
    const pkg = JSON.parse(read(`node_modules/${name}/package.json`))
    return [
      `${name} ${pkg.version} (${pkg.license})`,
      read(`node_modules/${name}/LICENSE`).trim(),
    ].join('\n\n')
  })
  const header = [
    'Fonts bundled with IRCord',
    '',
    'IRCord serves these typefaces itself from the Fontsource packages named below;',
    'no font is requested from a third party. Each is used under the license that follows it.',
  ].join('\n')
  return `${[header, ...sections].join(`\n\n${'='.repeat(72)}\n\n`)}\n`
}

function main() {
  mkdirSync(new URL('public/data/', frontend), { recursive: true })
  writeFileSync(noticePath, renderFontLicenses())
  console.log(`Wrote licenses for ${fontPackages().length} fonts.`)
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) main()
