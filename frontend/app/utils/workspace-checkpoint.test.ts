import assert from 'node:assert/strict'
import test from 'node:test'
import { writeCheckpoint, type WorkspaceCheckpoint } from './workspace-checkpoint'

const checkpoint = {
  version: 3,
  accountId: 'account',
  savedAt: '2026-01-01T00:00:00.000Z',
  bootstrap: {},
  messages: {},
} as unknown as WorkspaceCheckpoint

function withStorage(setItem: (key: string, value: string) => void, run: () => void) {
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: { setItem },
  })
  try {
    run()
  } finally {
    if (previous) Object.defineProperty(globalThis, 'localStorage', previous)
    else delete (globalThis as { localStorage?: unknown }).localStorage
  }
}

test('a stored checkpoint reports success', () => {
  const stored = new Map<string, string>()
  withStorage(
    (key, value) => stored.set(key, value),
    () => assert.equal(writeCheckpoint(checkpoint), true),
  )
  assert.ok(stored.has('ircord.snapshot.account'))
})

test('a full storage reports failure instead of throwing', () => {
  withStorage(
    () => {
      throw new DOMException('Quota exceeded', 'QuotaExceededError')
    },
    () => assert.equal(writeCheckpoint(checkpoint), false),
  )
})
