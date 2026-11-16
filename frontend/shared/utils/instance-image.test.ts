import assert from 'node:assert/strict'
import test from 'node:test'
import { instanceImageUrl } from './instance-image'

const origins = ['https://chat.example', 'https://api.chat.example']

test('pictures on this instance load', () => {
  assert.equal(
    instanceImageUrl('/avatars/defaults/fox.webp', origins),
    '/avatars/defaults/fox.webp',
  )
  assert.equal(
    instanceImageUrl('https://chat.example/avatars/defaults/fox.webp', origins),
    'https://chat.example/avatars/defaults/fox.webp',
  )
  const upload = `https://api.chat.example/files/${'a'.repeat(32)}/${'b'.repeat(48)}/me.png`
  assert.equal(instanceImageUrl(upload, origins), upload)
  // An image picked on this device, before it is saved.
  assert.equal(
    instanceImageUrl('data:image/png;base64,AAAA', origins),
    'data:image/png;base64,AAAA',
  )
  assert.equal(
    instanceImageUrl('blob:https://chat.example/1', origins),
    'blob:https://chat.example/1',
  )
})

test('pictures on any other host do not load', () => {
  assert.equal(instanceImageUrl('https://tracker.example/me.png', origins), undefined)
  assert.equal(instanceImageUrl('https://chat.example.tracker.example/me.png', origins), undefined)
  assert.equal(instanceImageUrl('//tracker.example/me.png', origins), undefined)
  // Another IRCord instance's upload is still another host.
  const remote = `https://other.example/files/${'a'.repeat(32)}/${'b'.repeat(48)}/me.png`
  assert.equal(instanceImageUrl(remote, origins), undefined)
  // Same host on another port or scheme is another origin.
  assert.equal(instanceImageUrl('https://chat.example:8443/me.png', origins), undefined)
  assert.equal(instanceImageUrl('http://chat.example/me.png', origins), undefined)
})

test('unusable values do not load', () => {
  assert.equal(instanceImageUrl(undefined, origins), undefined)
  assert.equal(instanceImageUrl('', origins), undefined)
  assert.equal(instanceImageUrl('https://user:pass@chat.example/me.png', origins), undefined)
  assert.equal(instanceImageUrl('javascript:alert(1)', origins), undefined)
  assert.equal(instanceImageUrl('data:text/html,hi', origins), undefined)
  assert.equal(instanceImageUrl('/me.png', []), undefined)
  assert.equal(instanceImageUrl('/me.png', ['not a url']), undefined)
})
