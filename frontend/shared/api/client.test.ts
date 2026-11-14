import assert from 'node:assert/strict'
import test from 'node:test'
import { ApiError } from './client'

const response = new Response(null, { status: 409, statusText: 'Conflict' })

test('ApiError exposes recognized problem types and retains status and detail', () => {
  const error = new ApiError(response, {
    type: 'urn:ircord:problem:delivery-uncertain',
    detail: 'Delivery needs reconciliation.',
    title: 'Conflict',
  })

  assert.equal(error.status, 409)
  assert.equal(error.statusCode, 409)
  assert.equal(error.code, 'delivery-uncertain')
  assert.equal(error.problemType, 'urn:ircord:problem:delivery-uncertain')
  assert.equal(error.data.detail, 'Delivery needs reconciliation.')
  assert.equal(error.message, 'Delivery needs reconciliation.')
})

test('ApiError keeps unknown and missing problem types generic', () => {
  const unknown = new ApiError(response, {
    type: 'urn:attacker:invented-code',
    detail: 'Safe detail from the response.',
  })
  const missing = new ApiError(response, { title: 'Conflict' })

  assert.equal(unknown.code, undefined)
  assert.equal(unknown.problemType, undefined)
  assert.equal(unknown.message, 'Safe detail from the response.')
  assert.equal(missing.code, undefined)
  assert.equal(missing.problemType, undefined)
  assert.equal(missing.message, 'Conflict')
})

test('ApiError does not recognize inherited lookup keys as problem types', () => {
  for (const type of ['constructor', 'toString', '__proto__']) {
    const error = new ApiError(response, { type, detail: 'Generic conflict.' })

    assert.equal(error.code, undefined, `${type} must not be a recognized code`)
    assert.equal(error.problemType, undefined, `${type} must not be exposed as a stable type`)
    assert.equal(error.message, 'Generic conflict.')
  }
})
