import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { interfaceFonts, messageFonts } from '../shared/appearance'
import {
  MAX_RESULTS,
  searchSettings,
  settingEntry,
  settingWords,
  settingsDestination,
  settingsIndex,
  settingsSections,
  type SettingsAccess,
} from '../shared/settings-index'

const user: SettingsAccess = { emoteManager: false, push: false, quietHours: false, install: false }
const everything: SettingsAccess = {
  emoteManager: true,
  push: true,
  quietHours: true,
  install: true,
}
const search = (query: string, access = user) => searchSettings(query, access).results
const ids = (query: string, access = user) => search(query, access).map((r) => r.entry.id)
const rankOf = (query: string, id: string, access = user) =>
  search(query, access).find((r) => r.entry.id === id)?.rank

/** The panel each component renders, so a row can be checked against its entry's section. */
const panels: Record<string, string> = {
  'AppearanceSettings.vue': 'appearance',
  'ProfileSettings.vue': 'profile',
  'NetworkSettingsList.vue': 'networks',
  'MessageSettings.vue': 'messages',
  'NotificationSettings.vue': 'notifications',
  'ReadAloudSettings.vue': 'read-aloud',
  'EmoteSettings.vue': 'emotes',
  'AccountSettings.vue': 'account',
  'ShortcutList.vue': 'shortcuts',
  'AboutSettings.vue': 'about',
}
/** Rows rendered on alternative v-if / v-else-if branches, so they appear more than once. */
const branches = new Set(['install-app'])
/** Whole lists whose visible text is the user's data (networks, shortcut bindings). */
const lists = new Set(['network-list', 'keyboard-shortcuts'])

interface Row {
  id: string
  file: string
  /** The row's markup, up to the next row, whitespace collapsed and lowercased. */
  text: string
}
function rows(): Row[] {
  const dir = join(import.meta.dirname, '../app/components')
  const found: Row[] = []
  for (const file of readdirSync(dir).filter((f) => f.endsWith('.vue'))) {
    const source = readFileSync(join(dir, file), 'utf8')
    const matches = [...source.matchAll(/data-setting="([\w-]+)"/g)]
    matches.forEach((match, i) =>
      found.push({
        id: match[1]!,
        file,
        text: source
          .slice(match.index, matches[i + 1]?.index)
          .replace(/\s+/g, ' ')
          .toLowerCase(),
      }),
    )
  }
  return found
}

test('every indexed setting has exactly one row in Settings, and every row is indexed', () => {
  const found = rows()
  const indexed = settingsIndex.map((e) => e.id)
  assert.equal(new Set(indexed).size, indexed.length, 'ids are unique')
  const unique = found.filter(
    (r, i) => !branches.has(r.id) || found.findIndex((o) => o.id === r.id) === i,
  )
  assert.deepEqual(unique.map((r) => r.id).sort(), [...indexed].sort())
})

test('each row sits in its entry’s section and shows its label or title', () => {
  for (const row of rows()) {
    const entry = settingEntry(row.id)!
    assert.equal(panels[row.file], entry.section, `${row.id} is rendered in ${row.file}`)
    if (lists.has(row.id)) continue
    const shown = [entry.label, entry.title]
      .filter((t): t is string => !!t)
      .some((t) => row.text.includes(t.toLowerCase()))
    assert.ok(shown, `${row.id} shows “${entry.label}”${entry.title ? ` or “${entry.title}”` : ''}`)
  }
})

test('no setting id is also a section id', () => {
  const sections = new Set<string>(settingsSections.map((s) => s.id))
  for (const entry of settingsIndex) assert.ok(!sections.has(entry.id), entry.id)
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

test('each ranking tier is applied', () => {
  // 3: the exact label (or on-screen title), whatever the case and spacing.
  assert.equal(rankOf('quiet hours', 'quiet-hours'), 3)
  assert.equal(rankOf('  QUIET  hours ', 'quiet-hours'), 3)
  assert.equal(rankOf('sound', 'notification-sound'), 2)
  // 2: every query word starts a word of the label.
  assert.equal(rankOf('font', 'message-font'), 2)
  // 1.5: an exact synonym.
  assert.equal(rankOf('font', 'interface-font'), 1.5)
  // 1: words found across label, synonyms, group and section.
  assert.equal(rankOf('conversation', 'channel-activity'), 1)
  assert.equal(rankOf('messages clock', 'clock'), 1)
  assert.equal(rankOf('motion gif', 'animate-media'), 1)
  // 0: only with the description.
  assert.equal(rankOf('gap before', 'group-spacing'), 0)

  const font = ids('font')
  assert.equal(font[0], 'message-font')
  assert.ok(font.indexOf('interface-font') > 0)
  for (const query of ['font', 'gif', 'notification', 'a']) {
    const ranks = search(query, everything).map((r) => r.rank)
    assert.deepEqual(
      ranks,
      [...ranks].sort((a, b) => b - a),
      query,
    )
  }
})

test('common queries find the setting people mean', () => {
  assert.ok(ids('GIFs').includes('animate-media'))
  assert.ok(!settingsIndex.some((entry) => entry.id === 'link-previews'))
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

  const [test] = search('test notification', user)
  assert.equal(test?.entry.id, 'notifications-test')
  assert.match(test?.unavailable ?? '', /push/)
  assert.equal(search('test notification', everything)[0]?.unavailable, undefined)

  // Usable results come before explained ones of the same rank, against Settings order.
  assert.equal(
    rankOf('notification', 'notifications-test'),
    rankOf('notification', 'notification-sound'),
  )
  const notification = ids('notification')
  assert.ok(notification.indexOf('notifications-test') > notification.indexOf('notification-sound'))
  assert.ok(
    ids('notification', everything).indexOf('notifications-test') <
      ids('notification', everything).indexOf('notification-sound'),
  )

  // The install row only exists while Settings offers it.
  assert.ok(!ids('install').includes('install-app'))
  assert.equal(ids('home screen', everything)[0], 'install-app')
})

test('destinations: sections, settings, and fallbacks for unavailable ones', () => {
  assert.deepEqual(settingsDestination('notifications', user), { section: 'notifications' })
  assert.deepEqual(settingsDestination('animate-media', user), {
    section: 'messages',
    setting: 'animate-media',
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
  assert.deepEqual(settingsDestination('network-list', user), {
    section: 'networks',
    setting: 'network-list',
  })
  assert.deepEqual(settingsDestination('no-such-thing', user), { section: 'appearance' })
})

test('the index holds static wording only', () => {
  const allowed = new Set([
    'id',
    'section',
    'group',
    'label',
    'title',
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
  assert.equal(search('x'.repeat(500), everything).length, 0)
  const many = searchSettings('a', everything)
  assert.ok(many.total > MAX_RESULTS, 'enough matches to hit the cap')
  assert.equal(many.results.length, MAX_RESULTS, 'results are bounded')
})

test('every typeface is found by its name', () => {
  for (const [entry, fonts, skip] of [
    ['interface-font', interfaceFonts, 'system'],
    ['message-font', messageFonts, 'interface'],
  ] as const) {
    for (const [id, font] of Object.entries(fonts)) {
      if (id === skip) continue
      assert.ok(ids(font.label).includes(entry), `${font.label} finds ${entry}`)
    }
  }
  assert.equal(ids('literata')[0], 'message-font')
  assert.equal(ids('source sans')[0], 'interface-font')
})
