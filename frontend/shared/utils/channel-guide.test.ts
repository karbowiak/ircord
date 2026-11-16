import assert from 'node:assert/strict'
import test from 'node:test'
import {
  guideHasChoices,
  guideSections,
  initialGuideSelection,
  type GuideChannel,
} from './channel-guide'

const channel = (name: string, state: GuideChannel['state'], extra: Partial<GuideChannel> = {}) =>
  ({ channel: name, state, nsfw: false, ...extra }) as GuideChannel

const channels = [
  channel('#food', 'listed', { group: 'Off topic' }),
  channel('#general', 'everyone', { group: 'Games' }),
  channel('#eve', 'suggested', { group: 'games' }),
  channel('#late', 'suggested', { nsfw: true }),
  channel('#lost', 'listed', { group: 'Removed group' }),
]

test('the picker lists channels for everyone first, then groups in order, then the rest', () => {
  const sections = guideSections(
    [{ name: 'Games' }, { name: 'Empty' }, { name: 'Off topic' }],
    channels,
  )
  assert.deepEqual(
    sections.map((s) => [s.kind, s.name, s.channels.map((c) => c.channel)]),
    [
      ['everyone', undefined, ['#general']],
      ['group', 'Games', ['#eve']],
      ['group', 'Off topic', ['#food']],
      ['more', undefined, ['#late', '#lost']],
    ],
  )
})

test('suggested channels start ticked, except sensitive ones and ones you are in', () => {
  assert.deepEqual(initialGuideSelection(channels, new Set()), ['#eve'])
  assert.deepEqual(initialGuideSelection(channels, new Set(['#eve'])), [])
})

test('a guide with only channels for everyone offers no choice', () => {
  assert.equal(guideHasChoices([channel('#general', 'everyone')], new Set()), false)
  assert.equal(guideHasChoices(channels, new Set(['#food', '#eve', '#late'])), true)
  assert.equal(guideHasChoices(channels, new Set(['#food', '#eve', '#late', '#lost'])), false)
})
