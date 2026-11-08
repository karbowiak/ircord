import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import {
  searchSettings,
  settingEntry,
  settingWords,
  settingsDestination,
  settingsIndex,
  settingsSections,
  type SettingsAccess,
} from '../shared/settings-index'

const user: SettingsAccess = { emoteManager: false, push: false, quietHours: false }
const everything: SettingsAccess = { emoteManager: true, push: true, quietHours: true }
const ids = (query: string, access = user) => searchSettings(query, access).map((r) => r.entry.id)

test('every indexed setting has exactly one row in Settings, and every row is indexed', () => {
  const dir = join(import.meta.dirname, '../app/components')
  const rows: string[] = []
  for (const file of readdirSync(dir).filter((f) => f.endsWith('.vue')))
    for (const match of readFileSync(join(dir, file), 'utf8').matchAll(/data-setting="([\w-]+)"/g))
      rows.push(match[1]!)
  const indexed = settingsIndex.map((e) => e.id)
  assert.equal(new Set(indexed).size, indexed.length, 'ids are unique')
  assert.deepEqual([...rows].sort(), [...indexed].sort())
})

test('entries name a real section, and fallbacks are always reachable', () => {
  const sections = new Set(settingsSections.map((s) => s.id))
  for (const entry of settingsIndex) {
    assert.ok(sections.has(entry.section), entry.id)
    if (entry.fallback) {
      const fallback = settingEntry(entry.fallback)
      assert.ok(fallback && !fallback.requires, `${entry.id} falls back to an available setting`)
    }
    if (entry.unavailable) assert.ok(entry.requires, `${entry.id} explains a requirement`)
  }
})

test('words ignore case, spacing, accents, punctuation and plurals', () => {
  assert.deepEqual(settingWords('  Quiet   HOURS '), ['quiet', 'hour'])
  assert.deepEqual(settingWords('GIFs/vidéos'), ['gif', 'video'])
  assert.deepEqual(settingWords('what’s new'), ['what', 'new'])
  assert.deepEqual(settingWords('   '), [])
})

test('exact labels rank first, then label words, then synonyms', () => {
  assert.equal(ids('quiet hours')[0], 'quiet-hours')
  assert.equal(ids('  QUIET  hours ')[0], 'quiet-hours')
  assert.equal(ids('accent')[0], 'accent')
  const font = ids('font')
  assert.equal(font[0], 'message-font')
  assert.ok(font.indexOf('interface-font') > 0)
  const ranks = searchSettings('font', user).map((r) => r.rank)
  assert.deepEqual(
    ranks,
    [...ranks].sort((a, b) => b - a),
  )
})

test('common queries find the setting people mean', () => {
  assert.equal(ids('autoplay')[0], 'autoplay-media')
  assert.ok(ids('gif').includes('autoplay-media'))
  assert.ok(ids('GIFs').includes('animate-media'))
  assert.ok(ids('video autoplay').includes('autoplay-media'))
  assert.equal(ids('image previews')[0], 'link-previews')
  assert.ok(ids('message fonts').includes('message-font'))
  assert.ok(ids('devices').includes('notifications-device'))
  assert.ok(ids('dnd').includes('quiet-hours'))
  assert.equal(ids('hotkeys')[0], 'keyboard-shortcuts')
  assert.ok(ids('dark mode').includes('theme-mode'))
  assert.ok(ids('avatar').includes('profile-image'))
  assert.deepEqual(ids('zzzz nothing'), [])
  assert.deepEqual(ids(''), [])
  assert.deepEqual(ids(' -- '), [])
})

test('settings the user cannot reach are hidden or explained', () => {
  assert.ok(!ids('add an emote').includes('add-emote'))
  assert.equal(ids('add an emote', everything)[0], 'add-emote')

  const [test] = searchSettings('test notification', user)
  assert.equal(test?.entry.id, 'notifications-test')
  assert.match(test?.unavailable ?? '', /push/)
  assert.equal(searchSettings('test notification', everything)[0]?.unavailable, undefined)

  // Usable results come before explained ones of the same rank.
  const quiet = searchSettings('quiet hours', user)
  assert.ok(quiet.findIndex((r) => r.unavailable) > quiet.findIndex((r) => !r.unavailable))
})

test('destinations: sections, settings, and fallbacks for unavailable ones', () => {
  assert.deepEqual(settingsDestination('notifications', user), { section: 'notifications' })
  assert.deepEqual(settingsDestination('autoplay-media', user), {
    section: 'notifications',
    setting: 'autoplay-media',
  })
  assert.deepEqual(settingsDestination('notifications-test', user), {
    section: 'notifications',
    setting: 'notifications-device',
  })
  assert.deepEqual(settingsDestination('notifications-test', everything), {
    section: 'notifications',
    setting: 'notifications-test',
  })
  assert.deepEqual(settingsDestination('add-emote', user), { section: 'emotes' })
  assert.deepEqual(settingsDestination('no-such-thing', user), { section: 'appearance' })
})

test('the index holds static wording only', () => {
  const allowed = new Set([
    'id',
    'section',
    'group',
    'label',
    'description',
    'keywords',
    'requires',
    'unavailable',
    'fallback',
  ])
  for (const entry of settingsIndex) {
    for (const [key, value] of Object.entries(entry)) {
      assert.ok(allowed.has(key), `${entry.id}.${key}`)
      assert.ok(typeof value === 'string' || Array.isArray(value), `${entry.id}.${key} is text`)
    }
  }
  assert.ok(searchSettings('x'.repeat(500), everything).length === 0)
  assert.ok(searchSettings('e', everything).length <= 20, 'results are bounded')
})
