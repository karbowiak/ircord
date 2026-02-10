export type EmojiCategoryId = 'people' | 'nature' | 'food' | 'activity' | 'objects' | 'symbols' | 'custom'

export interface EmojiDefinition {
  shortcode: string
  char?: string
  imageUrl?: string
  category: EmojiCategoryId
  keywords: string[]
}

export interface EmojiCategory {
  id: EmojiCategoryId
  label: string
}

export const emojiCategories: EmojiCategory[] = [
  { id: 'people', label: 'Smileys & People' },
  { id: 'nature', label: 'Animals & Nature' },
  { id: 'food', label: 'Food & Drink' },
  { id: 'activity', label: 'Activities' },
  { id: 'objects', label: 'Objects' },
  { id: 'symbols', label: 'Symbols' },
  { id: 'custom', label: 'Custom' },
]

export const emojiDefinitions: EmojiDefinition[] = [
  { shortcode: 'wave', char: '👋', category: 'people', keywords: ['hello', 'hi', 'bye', 'hand'] },
  { shortcode: 'smileyface', char: '😃', category: 'people', keywords: ['smile', 'happy', 'grin'] },
  { shortcode: 'grin', char: '😁', category: 'people', keywords: ['smile', 'happy'] },
  { shortcode: 'joy', char: '😂', category: 'people', keywords: ['laugh', 'tears'] },
  { shortcode: 'rofl', char: '🤣', category: 'people', keywords: ['laugh', 'rolling'] },
  { shortcode: 'heart_eyes', char: '😍', category: 'people', keywords: ['love', 'eyes'] },
  { shortcode: 'thinking', char: '🤔', category: 'people', keywords: ['hmm', 'think'] },
  { shortcode: 'sunglasses', char: '😎', category: 'people', keywords: ['cool', 'summer'] },
  { shortcode: 'shrug', char: '🤷', category: 'people', keywords: ['idk', 'meh'] },
  { shortcode: 'thumbsup', char: '👍', category: 'people', keywords: ['ok', 'yes'] },
  { shortcode: 'fire', char: '🔥', category: 'nature', keywords: ['lit', 'hot'] },
  { shortcode: 'sparkles', char: '✨', category: 'nature', keywords: ['shiny', 'magic'] },
  { shortcode: 'sun', char: '☀️', category: 'nature', keywords: ['weather', 'day'] },
  { shortcode: 'moon', char: '🌙', category: 'nature', keywords: ['night'] },
  { shortcode: 'cat', char: '🐱', category: 'nature', keywords: ['pet', 'kitty'] },
  { shortcode: 'dog', char: '🐶', category: 'nature', keywords: ['pet', 'puppy'] },
  { shortcode: 'pizza', char: '🍕', category: 'food', keywords: ['food', 'slice'] },
  { shortcode: 'coffee', char: '☕', category: 'food', keywords: ['drink', 'cafe'] },
  { shortcode: 'beer', char: '🍺', category: 'food', keywords: ['drink', 'party'] },
  { shortcode: 'cake', char: '🎂', category: 'food', keywords: ['birthday', 'dessert'] },
  { shortcode: 'party', char: '🥳', category: 'activity', keywords: ['celebrate'] },
  { shortcode: 'clap', char: '👏', category: 'activity', keywords: ['applause'] },
  { shortcode: 'rocket', char: '🚀', category: 'activity', keywords: ['ship', 'launch'] },
  { shortcode: 'soccer', char: '⚽', category: 'activity', keywords: ['sports'] },
  { shortcode: 'laptop', char: '💻', category: 'objects', keywords: ['computer', 'dev'] },
  { shortcode: 'phone', char: '📱', category: 'objects', keywords: ['mobile'] },
  { shortcode: 'bulb', char: '💡', category: 'objects', keywords: ['idea'] },
  { shortcode: 'lock', char: '🔒', category: 'objects', keywords: ['secure'] },
  { shortcode: 'heart', char: '❤️', category: 'symbols', keywords: ['love'] },
  { shortcode: 'check', char: '✅', category: 'symbols', keywords: ['done', 'yes'] },
  { shortcode: 'x', char: '❌', category: 'symbols', keywords: ['no', 'wrong'] },
  { shortcode: 'warning', char: '⚠️', category: 'symbols', keywords: ['alert'] },
  {
    shortcode: 'kekw',
    imageUrl: 'https://static-cdn.jtvnw.net/emoticons/v2/425618/default/dark/3.0',
    category: 'custom',
    keywords: ['meme', 'laugh', 'twitch'],
  },
]

export const emoticonAliases: Array<[string, string]> = [
  [':-D', '😄'],
  [':D', '😄'],
  ['xD', '😂'],
  ['XD', '😂'],
  [':-)', '🙂'],
  [':)', '🙂'],
  [':-(', '🙁'],
  [':(', '🙁'],
  [';-)', '😉'],
  [';)', '😉'],
]
