import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  deviceStatus,
  pushPossible,
  sanitizeDeviceMode,
  sanitizeSetupRecord,
  setupOffer,
  type DeviceFacts,
  type OfferInput,
} from '../shared/notification-setup'

/** An Android phone in Chrome on a server with push, nothing set up yet. */
const facts = (changes: Partial<DeviceFacts> = {}): DeviceFacts => ({
  notifications: true,
  push: true,
  needsInstall: false,
  permission: 'default',
  mode: 'off',
  serverPush: true,
  browserSubscribed: false,
  serverRegistered: false,
  ...changes,
})
const offer = (f: Partial<DeviceFacts> = {}, input: Partial<Omit<OfferInput, 'facts'>> = {}) =>
  setupOffer({
    facts: facts(f),
    eligible: true,
    chosen: false,
    record: null,
    online: true,
    ...input,
  })
const working = {
  permission: 'granted',
  mode: 'push',
  browserSubscribed: true,
  serverRegistered: true,
} as const

test('the summary tells a granted permission apart from a working push registration', () => {
  assert.equal(deviceStatus(facts(working)), 'push')
  assert.equal(deviceStatus(facts({ ...working, serverRegistered: false })), 'repair')
  assert.equal(deviceStatus(facts({ ...working, browserSubscribed: false })), 'repair')
  assert.equal(deviceStatus(facts({ ...working, serverRegistered: undefined })), 'checking')
  assert.equal(deviceStatus(facts({ ...working, serverPush: false })), 'server-unavailable')
  // Permission alone is not "on".
  assert.equal(deviceStatus(facts({ permission: 'granted' })), 'off')
  assert.equal(deviceStatus(facts({ permission: 'granted', mode: 'open' })), 'open')
})

test('blocked, unsupported, install-first and reset permission each have their own state', () => {
  assert.equal(deviceStatus(facts({ permission: 'denied', mode: 'push' })), 'blocked')
  assert.equal(
    deviceStatus(facts({ notifications: false, permission: 'unsupported' })),
    'unsupported',
  )
  assert.equal(
    deviceStatus(facts({ needsInstall: true, notifications: false, permission: 'unsupported' })),
    'install',
  )
  // Chosen earlier, then the permission was reset in the browser's settings.
  assert.equal(deviceStatus(facts({ mode: 'push', permission: 'default' })), 'permission')
})

test('situation: eligible app, permission not decided → Enable / Not now', () => {
  assert.equal(offer(), 'enable')
  assert.equal(offer({}, { record: 'not-now' }), null, 'Not now is remembered')
})

test('situation: permission granted but setup incomplete → repair, without asking again', () => {
  assert.equal(offer({ ...working, serverRegistered: false }), 'repair')
  assert.equal(
    offer({ ...working, browserSubscribed: false }, { record: 'enabled', chosen: true }),
    'repair',
  )
  // Granted but never chosen: an ordinary offer to turn it on (no native prompt is needed).
  assert.equal(offer({ permission: 'granted' }), 'enable')
})

test('situation: already enabled and configured → no prompt', () => {
  assert.equal(offer(working, { chosen: true }), null)
  assert.equal(offer({ permission: 'granted', mode: 'open' }, { chosen: true }), null)
  // Still confirming the registration is not a reason to nag.
  assert.equal(offer({ ...working, serverRegistered: undefined }, { chosen: true }), null)
})

test('situation: denied or a deliberate Off is respected', () => {
  assert.equal(offer({ permission: 'denied' }), null)
  assert.equal(offer({ permission: 'denied', mode: 'push' }, { chosen: true }), null)
  assert.equal(offer({}, { chosen: true }), null, 'a stored Off is a choice, not the default')
  assert.equal(offer({}, { chosen: false }), 'enable', 'the default Off is not')
})

test('situation: installation required → install help once, then continue in the app', () => {
  const tab = { needsInstall: true, notifications: false, permission: 'unsupported' } as const
  assert.equal(offer(tab), 'install')
  assert.equal(offer(tab, { record: 'not-now' }), null)
  // The Home Screen app starts with its own storage, so no record: setup is offered there.
  assert.equal(offer({ permission: 'default' }), 'enable')
})

test('situation: unsupported, server without push, offline or uncertain → defer', () => {
  assert.equal(offer({ notifications: false, permission: 'unsupported' }), null)
  assert.equal(offer({ serverPush: false }), null)
  assert.equal(offer({ serverPush: undefined }), null, 'unknown server support')
  assert.equal(offer({ push: false }), null, 'no false “enabled” through while-open only')
  assert.equal(offer({}, { online: false }), null)
  assert.equal(offer({ ...working, serverRegistered: false }, { online: false }), null)
})

test('a desktop browser tab is never prompted; Settings remains the place to set up', () => {
  assert.equal(offer({}, { eligible: false }), null)
  assert.equal(offer({ ...working, serverRegistered: false }, { eligible: false }), null)
})

test('Enable from the prompt only counts once; a later broken registration is offered once more', () => {
  assert.equal(offer({}, { record: 'enabled' }), null)
  assert.equal(
    offer({ ...working, serverRegistered: false }, { record: 'enabled', chosen: true }),
    'repair',
  )
  assert.equal(
    offer({ ...working, serverRegistered: false }, { record: 'not-now', chosen: true }),
    null,
  )
})

test('push is only possible with browser and server support outside an iOS browser tab', () => {
  assert.equal(pushPossible(facts()), true)
  assert.equal(pushPossible(facts({ serverPush: undefined })), false)
  assert.equal(pushPossible(facts({ needsInstall: true })), false)
  assert.equal(pushPossible(facts({ push: false })), false)
})

test('stored choices are sanitised', () => {
  assert.equal(sanitizeDeviceMode('push'), 'push')
  assert.equal(sanitizeDeviceMode(undefined), null)
  assert.equal(sanitizeDeviceMode('always'), null)
  assert.equal(sanitizeSetupRecord('not-now'), 'not-now')
  assert.equal(sanitizeSetupRecord(true), null)
})

test('push that cannot work here says why before asking for permission', () => {
  // No Web Push in this browser: not “lost its registration” with no way to repair.
  assert.equal(deviceStatus(facts({ ...working, push: false })), 'push-unsupported')
  assert.equal(
    deviceStatus(facts({ mode: 'push', permission: 'default', push: false })),
    'push-unsupported',
  )
  // A server without push: not “allow notifications” first, only to fail after granting.
  assert.equal(
    deviceStatus(facts({ mode: 'push', permission: 'default', serverPush: false })),
    'server-unavailable',
  )
  assert.equal(
    offer({ mode: 'push', permission: 'default', serverPush: false }, { chosen: true }),
    null,
  )
  assert.equal(offer({ ...working, push: false }, { chosen: true }), null)
})
