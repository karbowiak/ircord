import { writeFileSync, mkdirSync, copyFileSync } from 'node:fs'
import { createRequire } from 'node:module'
const require = createRequire(import.meta.url)
const raw = require('emojibase-data/en/data.json')
const codes = require('emojibase-data/en/shortcodes/iamcal.json')
const cldr = require('emojibase-data/en/shortcodes/cldr.json')
const groups = require('emojibase-data/en/messages.json').groups
// Newer entries can render as missing-glyph bars in current native emoji fonts.
// Keep the picker on the supported Emoji 16 repertoire; messages remain Unicode.
const items = raw
  .filter((item) => typeof item.group === 'number' && item.group !== 2 && item.version <= 16)
  .map((item) => {
    const aliases = [codes[item.hexcode], cldr[item.hexcode], ...(item.tags || [])]
      .flat()
      .filter((alias) => typeof alias === 'string')
    return {
      id: item.hexcode,
      label: item.label,
      shortcode: aliases[0] || item.label.replace(/[^a-z0-9]+/gi, '_'),
      aliases,
      group: groups.find((group) => group.order === item.group)?.key || 'symbols',
      unicode: item.emoji,
      source: 'unicode',
      ...(item.skins?.length
        ? {
            variants: Object.fromEntries(
              item.skins.map((skin) => [
                String(Array.isArray(skin.tone) ? skin.tone[0] : skin.tone),
                skin.emoji,
              ]),
            ),
          }
        : {}),
    }
  })
mkdirSync('public/data', { recursive: true })
writeFileSync('public/data/emoji-v18.json', JSON.stringify(items))
copyFileSync('node_modules/emojibase-data/LICENSE', 'public/data/emojibase-LICENSE.txt')
console.log(`Generated ${items.length} Emoji 16-compatible entries from Emojibase 18.`)
