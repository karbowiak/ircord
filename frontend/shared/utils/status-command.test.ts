import assert from 'node:assert/strict'
import test from 'node:test'
import { parseStatusCommand, runStatusCommand, type StatusUpdate } from './status-command'

const now = Date.parse('2026-11-14T12:00:00Z')
const set = (argument: string) => parseStatusCommand(argument, now)

test('opens the editor or clears only status fields', () => {
  assert.deepEqual(set('  '), { kind: 'edit' })
  assert.deepEqual(set('clear'), {
    kind: 'clear',
    input: { statusText: '', statusEmoji: '', statusExpiresAt: '' },
  })
})

test('parses emoji graphemes without consuming ordinary words or digits', () => {
  for (const emoji of ['☕', '☕️', '🇩🇰', '👩🏽‍💻', '1️⃣']) {
    assert.deepEqual(set(`${emoji} In a meeting`), {
      kind: 'set',
      input: { statusText: 'In a meeting', statusEmoji: emoji, statusExpiresAt: '' },
    })
  }
  for (const message of ['1 meeting', 'hello world', 'coffee break', '☕coffee break']) {
    assert.deepEqual(set(message), {
      kind: 'set',
      input: { statusText: message, statusEmoji: '', statusExpiresAt: '' },
    })
  }
  assert.deepEqual(set('--emoji :coffee: In a meeting --for 1h'), {
    kind: 'set',
    input: {
      statusText: 'In a meeting',
      statusEmoji: ':coffee:',
      statusExpiresAt: '2026-11-14T13:00:00.000Z',
    },
  })
})

test('accepts positive bounded durations and never', () => {
  for (const [duration, until] of [
    ['1m', '2026-11-14T12:01:00.000Z'],
    ['2h', '2026-11-14T14:00:00.000Z'],
    ['365d', '2027-11-14T12:00:00.000Z'],
    ['never', ''],
  ]) {
    const result = set(`Busy --for ${duration}`)
    assert.equal(result.kind, 'set')
    if (result.kind === 'set') assert.equal(result.input.statusExpiresAt, until)
  }
  for (const duration of ['0m', '366d', '8761h', '1.5h', '-1h', '1s', '999999999999999999d'])
    assert.equal(set(`Busy --for ${duration}`).kind, 'invalid', duration)
})

test('rejects incomplete, duplicate and unknown flags without treating them as messages', () => {
  for (const value of [
    '--for',
    '--emoji',
    '--emoji :coffee:',
    'Busy --for --emoji x',
    'Busy --for 1h --for 2h',
    'Busy --emoji a --emoji b',
    '☕ Busy --emoji :coffee:',
    'Busy --other x',
    'Busy --for=1h',
    'Busy\nnow',
    'Busy\ud800',
  ])
    assert.equal(set(value).kind, 'invalid', value)
})

test('enforces UTF-8 byte limits for both fields', () => {
  assert.equal(set('é'.repeat(100)).kind, 'set')
  assert.equal(set('é'.repeat(101)).kind, 'invalid')
  assert.equal(set(`--emoji ${'é'.repeat(32)} Busy`).kind, 'set')
  assert.equal(set(`--emoji ${'é'.repeat(33)} Busy`).kind, 'invalid')
})

test('execution targets the captured network, opens the editor and changes only status', async () => {
  let currentNetwork = 'first'
  const updates: { id: string; input: StatusUpdate }[] = []
  const notices: string[] = []
  const edits: string[] = []
  const deps = {
    edit: (id: string) => (edits.push(id), true),
    update: async (id: string, input: StatusUpdate) => {
      currentNetwork = 'second'
      await Promise.resolve()
      updates.push({ id, input })
    },
    notify: (key: string) => notices.push(key),
  }
  assert.equal(await runStatusCommand('', currentNetwork, deps), 'handled')
  assert.deepEqual(edits, ['first'])
  assert.equal(await runStatusCommand('Busy', currentNetwork, deps), 'handled')
  assert.equal(await runStatusCommand('clear', currentNetwork, deps), 'handled')
  assert.deepEqual(updates, [
    { id: 'first', input: { statusText: 'Busy', statusEmoji: '', statusExpiresAt: '' } },
    { id: 'second', input: { statusText: '', statusEmoji: '', statusExpiresAt: '' } },
  ])
  assert.deepEqual(notices, ['set', 'cleared'])
})

test('execution keeps invalid drafts and unavailable editors; mutation errors propagate', async () => {
  const notices: string[] = []
  let updates = 0
  const deps = {
    edit: () => false,
    update: async () => {
      updates++
      throw new Error('offline')
    },
    notify: (key: string) => notices.push(key),
  }
  assert.equal(await runStatusCommand('Busy --for', 'n', deps), 'kept')
  assert.equal(await runStatusCommand('Busy', undefined, deps), 'kept')
  assert.equal(await runStatusCommand('', 'n', deps), 'kept')
  assert.equal(updates, 0)
  await assert.rejects(runStatusCommand('Busy', 'n', deps), /offline/)
  assert.deepEqual(notices, ['usage', 'network', 'editor'])
})
