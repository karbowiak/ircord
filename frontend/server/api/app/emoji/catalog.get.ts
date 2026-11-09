import workspace from '../../../assets/custom-emojis.json'
import fallback from '../../../assets/7tv-global.json'
import type { EmojiItem } from '../../../../shared/types/media'

interface SevenTvFile {
  name: string
  static_name?: string
}
interface SevenTvEmote {
  id: string
  name: string
  data?: { host?: { url?: string; files: SevenTvFile[] }; owner?: { display_name?: string } }
}

const sevenTv = defineCachedFunction(
  async (): Promise<EmojiItem[]> => {
    try {
      const raw = await $fetch<{ emotes: SevenTvEmote[] }>(
        'https://api.7tv.app/v3/emote-sets/global',
        {
          timeout: 5000,
          retry: 0,
        },
      )
      return raw.emotes
        .filter((emote) => /^[\w-]{1,64}$/.test(emote.name))
        .slice(0, 100)
        .map((emote) => {
          const host = emote.data?.host
          const base = `https:${host?.url}`
          if (!host || !base.startsWith('https://cdn.7tv.app/emote/'))
            throw new Error('Invalid emote host')
          const file =
            host.files.find((f) => f.name === '2x.webp') ??
            host.files.find((f) => f.name === '1x.webp')
          if (!file || !/^[\w-]{1,64}$/.test(emote.name)) throw new Error('Invalid emote')
          return {
            id: `7tv:${emote.id}`,
            label: emote.name,
            shortcode: emote.name,
            aliases: [],
            group: '7tv',
            url: `${base}/${file.name}`,
            staticUrl: `${base}/${file.static_name ?? file.name}`,
            source: '7tv',
            sourceUrl: `https://7tv.app/emotes/${emote.id}`,
            creator: emote.data?.owner?.display_name,
          }
        })
    } catch {
      return fallback as EmojiItem[]
    }
  },
  { maxAge: 3600, name: 'public-7tv-global' },
)

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'public, max-age=600')
  return {
    items: [...workspace, ...(await sevenTv())],
    standardUrl: '/data/emoji-v18.json',
    sources: ['workspace', '7tv'],
  }
})
