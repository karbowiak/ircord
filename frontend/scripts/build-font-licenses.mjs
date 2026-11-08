// Collects the license of every self-hosted Fontsource font into one public notice, so the font
// files the app serves travel with their copyright and license text (SIL OFL 1.1 requires it).
// Run after adding or updating a font package: node scripts/build-font-licenses.mjs
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'

const read = (path) => readFileSync(path, 'utf8')
const fonts = Object.keys(JSON.parse(read('package.json')).dependencies)
  .filter((name) => name.startsWith('@fontsource'))
  .sort()
const sections = fonts.map((name) => {
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
mkdirSync('public/data', { recursive: true })
writeFileSync(
  'public/data/font-licenses.txt',
  `${[header, ...sections].join(`\n\n${'='.repeat(72)}\n\n`)}\n`,
)
console.log(`Wrote licenses for ${fonts.length} fonts.`)
