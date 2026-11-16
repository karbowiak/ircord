import assert from 'node:assert/strict'
import test from 'node:test'
import { sharedCollator, sharedListFormat, sharedNumberFormat } from './intl'

test('number formats are shared per locale and options', () => {
  const format = sharedNumberFormat('en')
  assert.equal(sharedNumberFormat('en'), format)
  assert.equal(format.format(12000), '12,000')
})

test('collators are shared per locale and options', () => {
  const numeric = sharedCollator(undefined, { numeric: true, sensitivity: 'base' })
  assert.equal(sharedCollator(undefined, { numeric: true, sensitivity: 'base' }), numeric)
  assert.notEqual(sharedCollator('en'), numeric)
  assert.ok(numeric.compare('chan2', 'chan10') < 0)
  assert.equal(numeric.compare('Chan', 'chan'), 0)
})

test('list formats are shared per locale and options', () => {
  const list = sharedListFormat('en', { type: 'conjunction' })
  assert.equal(sharedListFormat('en', { type: 'conjunction' }), list)
  assert.equal(list.format(['Mark unread', 'Delete']), 'Mark unread and Delete')
})
