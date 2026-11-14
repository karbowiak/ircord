import assert from 'node:assert/strict'
import test from 'node:test'
import {
  fieldChanges,
  mergeFields,
  mergeProfileFields,
  profileChanges,
  type ProfileFields,
} from './profile-draft'

const baseline: ProfileFields = { name: 'Alice', bio: 'Old bio', color: null }

test('authoritative updates refresh untouched fields and preserve dirty fields', () => {
  const draft = { ...baseline, bio: 'My draft' }
  const remote = { name: 'Remote name', bio: 'Remote bio', color: '#123456' }
  const merged = mergeProfileFields(baseline, draft, remote)
  assert.deepEqual(merged, { name: 'Remote name', bio: 'My draft', color: '#123456' })
  assert.deepEqual(profileChanges(remote, merged), { bio: 'My draft' })
})

test('only explicitly changed fields are sent, including clears', () => {
  assert.deepEqual(profileChanges(baseline, { ...baseline, name: ' Alice ' }), {})
  assert.deepEqual(profileChanges(baseline, { ...baseline, name: '', color: '#123456' }), {
    displayName: '',
    nameColor: '#123456',
  })
  assert.deepEqual(profileChanges({ ...baseline, color: '#123456' }, { ...baseline, bio: '' }), {
    bio: '',
    nameColor: null,
  })
})

test('input edited during a save remains even when it equals the old baseline', () => {
  const server = { ...baseline, name: 'Submitted' }
  const typedDuringSave = { ...baseline }
  const merged = mergeProfileFields(baseline, typedDuringSave, server, { name: true })
  assert.equal(merged.name, 'Alice')
  assert.deepEqual(profileChanges(server, merged), { displayName: 'Alice' })
})

test('a submitted field settles to newer canonical state unless edited again', () => {
  const submitted = { ...baseline, bio: 'Submitted A' }
  const newer = { ...baseline, bio: 'Newer B' }
  assert.equal(mergeProfileFields(baseline, submitted, newer, {}, { bio: true }).bio, 'Newer B')
  assert.equal(
    mergeProfileFields(
      baseline,
      { ...submitted, bio: 'Typed later' },
      newer,
      { bio: true },
      { bio: true },
    ).bio,
    'Typed later',
  )
})

const details = { pronouns: '', interests: ['IRC'], statusText: '' }

test('detail fields send only what changed, lists compared by value', () => {
  assert.deepEqual(fieldChanges(details, { ...details, interests: ['IRC'] }), {})
  assert.deepEqual(fieldChanges(details, { ...details, pronouns: 'she/her', interests: [] }), {
    pronouns: 'she/her',
    interests: [],
  })
})

test('detail fields keep local edits and take remote updates for the rest', () => {
  const draft = { ...details, pronouns: 'they/them' }
  const remote = { pronouns: 'he/him', interests: ['Music'], statusText: 'lunch' }
  assert.deepEqual(mergeFields(details, draft, remote), {
    pronouns: 'they/them',
    interests: ['Music'],
    statusText: 'lunch',
  })
  assert.deepEqual(mergeFields(details, details, remote, { statusText: true }).statusText, '')
})
