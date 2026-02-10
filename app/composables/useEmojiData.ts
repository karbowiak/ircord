import {
  emojiCategories,
  emojiDefinitions,
  emoticonAliases,
} from '~/mocks/emoji'

export {
  emojiCategories,
  emojiDefinitions,
}

export type {
  EmojiCategory,
  EmojiDefinition,
} from '~/mocks/emoji'

const shortcodeMap = new Map(
  emojiDefinitions.map((entry) => [entry.shortcode.toLowerCase(), entry])
)

shortcodeMap.set('smile', shortcodeMap.get('smileyface')!)
shortcodeMap.set('happy', shortcodeMap.get('smileyface')!)

export function normalizeShortcode(input: string): string {
  return input.trim().replace(/^:/, '').replace(/:$/, '').toLowerCase()
}

export function findEmojiByShortcode(input: string): EmojiDefinition | undefined {
  return shortcodeMap.get(normalizeShortcode(input))
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export function normalizeEmoticonsInText(input: string): string {
  let output = input

  for (const [alias, emoji] of emoticonAliases) {
    const pattern = new RegExp(`(^|\\s)${escapeRegExp(alias)}(?=\\s|$|[.,!?])`, 'g')
    output = output.replace(pattern, `$1${emoji}`)
  }

  return output
}
