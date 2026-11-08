import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  customShortcodes,
  emojiText,
  instanceEmojiItems,
  parseProviderEmote,
  searchEmoji,
} from '../shared/utils/media'
import type { EmojiItem, InstanceCatalog, InstanceEmote } from '../shared/types/media'

const emote = (over: Partial<InstanceEmote>): InstanceEmote => ({
  id: 'abc',
  name: 'KEKW',
  shortcode: 'kekw',
  aliases: ['KEKW'],
  retired: [],
  enabled: true,
  animated: false,
  url: 'https://chat.example/emotes/a.webp',
  staticUrl: 'https://chat.example/emotes/a.webp',
  width: 64,
  height: 64,
  source: '7tv',
  providerId: '01GA29CZ2R000C36HNE7Z0DQXD',
  sourceUrl: 'https://7tv.app/emotes/01GA29CZ2R000C36HNE7Z0DQXD',
  creator: 'moecoquito',
  createdAt: '2026-11-08T00:00:00Z',
  updatedAt: '2026-11-08T00:00:00Z',
  ...over,
})
const catalog = (items: InstanceEmote[]): InstanceCatalog => ({
  version: 3,
  canManage: false,
  maxBytes: 1 << 20,
  types: ['image/png'],
  providers: ['7tv', 'ffz'],
  items,
})
const bundled = (source: '7tv' | 'workspace', shortcode: string): EmojiItem => ({
  id: `${source}:${shortcode}`,
  label: shortcode,
  shortcode,
  aliases: [],
  group: source,
  url: `https://cdn.example/${source}/${shortcode}`,
  source,
})

test('instance emotes become picker entries in the Server emoji group with attribution', () => {
  const items = instanceEmojiItems(
    catalog([emote({}), emote({ id: 'off', shortcode: 'gone', enabled: false })]),
  )
  assert.equal(items.length, 1, 'disabled entries are not offered')
  const [kekw] = items
  assert.equal(kekw!.id, 'instance:abc')
  assert.equal(kekw!.group, 'workspace')
  assert.equal(kekw!.source, 'instance')
  assert.equal(kekw!.provider, '7tv')
  assert.equal(kekw!.label, 'KEKW')
  assert.equal(kekw!.creator, 'moecoquito')
  // Sent as the readable shortcode, which is all other IRC clients see.
  assert.equal(emojiText(kekw!), ':kekw:')
  assert.deepEqual(
    searchEmoji(items, 'kek').map((i) => i.id),
    ['instance:abc'],
  )
  assert.equal(instanceEmojiItems(catalog([emote({ source: 'upload' })]))[0]!.provider, undefined)
  assert.deepEqual(instanceEmojiItems(undefined), [])
})

test('the backend catalog wins name clashes, and retired names only fill gaps', () => {
  const data = catalog([
    emote({}),
    emote({
      id: 'wave',
      name: 'waving',
      shortcode: 'waving',
      aliases: [],
      retired: ['wave', 'kekw', 'pog'],
    }),
  ])
  const instance = instanceEmojiItems(data)
  const map = customShortcodes(
    [
      bundled('7tv', 'kekw'),
      bundled('7tv', 'pog'),
      bundled('workspace', 'kekw'),
      bundled('workspace', 'ircord'),
    ],
    instance,
    data,
  )
  assert.equal(map.get('kekw')?.id, 'instance:abc', 'instance beats workspace and 7TV')
  assert.equal(map.get('KEKW')?.id, 'instance:abc', 'canonical casing works as an alias')
  assert.equal(map.get('waving')?.id, 'instance:wave')
  assert.equal(map.get('wave')?.id, 'instance:wave', 'old messages keep rendering after a rename')
  assert.equal(map.get('pog')?.id, '7tv:pog', 'a retired name never shadows a live one')
  assert.equal(map.get('ircord')?.id, 'workspace:ircord', 'existing workspace entries stay')
  assert.equal(
    customShortcodes([bundled('7tv', 'x'), bundled('workspace', 'x')], [], undefined).get('x')
      ?.source,
    'workspace',
  )
})

test('provider links and IDs are read into import requests', () => {
  assert.deepEqual(parseProviderEmote('https://7tv.app/emotes/01GA29CZ2R000C36HNE7Z0DQXD'), {
    provider: '7tv',
    providerId: '01GA29CZ2R000C36HNE7Z0DQXD',
  })
  assert.deepEqual(parseProviderEmote('01ga29cz2r000c36hne7z0dqxd'), {
    provider: '7tv',
    providerId: '01GA29CZ2R000C36HNE7Z0DQXD',
  })
  assert.deepEqual(parseProviderEmote('https://www.frankerfacez.com/emoticon/407009-KEKL'), {
    provider: 'ffz',
    providerId: '407009',
  })
  assert.deepEqual(parseProviderEmote('481082'), { provider: 'ffz', providerId: '481082' })
  assert.equal(
    parseProviderEmote('https://evil.example/emotes/01GA29CZ2R000C36HNE7Z0DQXD'),
    undefined,
  )
  assert.equal(parseProviderEmote('KEKW'), undefined)
})
