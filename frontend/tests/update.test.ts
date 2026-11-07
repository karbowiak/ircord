import { test } from 'node:test'
import assert from 'node:assert/strict'
import { canReloadSilently, isNewerBuild, type ReloadState } from '../shared/utils/update'

test('a different build id in the manifest is a newer build', () => {
  assert.equal(isNewerBuild({ id: 'a' }, { id: 'b', timestamp: 1 }), true)
  assert.equal(isNewerBuild({ id: 'a' }, { id: 'a', timestamp: 1 }), false)
  assert.equal(isNewerBuild({ id: 'a', timestamp: 5 }, { id: 'b', timestamp: 9 }), true)
  assert.equal(isNewerBuild({ id: 'a', timestamp: 5 }, { id: 'b' }), true)
})

test('an older build (a replica not yet updated) is not an update', () => {
  assert.equal(isNewerBuild({ id: 'new', timestamp: 9 }, { id: 'old', timestamp: 5 }), false)
})

test('malformed or empty manifests never report an update', () => {
  assert.equal(isNewerBuild({ id: 'a' }, { id: '' }), false)
  assert.equal(isNewerBuild({ id: 'a' }, { timestamp: 1 }), false)
  assert.equal(isNewerBuild({ id: 'a' }, { id: 42 }), false)
  assert.equal(isNewerBuild({ id: 'a' }, null), false)
  assert.equal(isNewerBuild({ id: 'a' }, 'b'), false)
  assert.equal(isNewerBuild({ id: '' }, { id: 'b' }), false)
})

test('only a long-hidden tab with nothing unsaved reloads without asking', () => {
  const idle: ReloadState = {
    hidden: true,
    hiddenLongEnough: true,
    outboxEmpty: true,
    uploadsPending: false,
    overlayOpen: false,
    unsavedInput: false,
  }
  assert.equal(canReloadSilently(idle), true)
  const blockers: Partial<ReloadState>[] = [
    { hidden: false },
    { hiddenLongEnough: false },
    { outboxEmpty: false },
    { uploadsPending: true },
    { overlayOpen: true },
    { unsavedInput: true },
  ]
  for (const blocker of blockers)
    assert.equal(canReloadSilently({ ...idle, ...blocker }), false, JSON.stringify(blocker))
})
