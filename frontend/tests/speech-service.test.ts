import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  sanitizeSpeechKey,
  sanitizeSpeechService,
  speechConfigured,
  speechFailure,
  speechOrigin,
  speechRequest,
} from '../shared/speech-service'
import { defaultPreferences, sanitizePreferences } from '../shared/preferences'

test('the speech origin is scheme, host and port only', () => {
  assert.equal(speechOrigin('https://api.openai.com/v1'), 'https://api.openai.com')
  assert.equal(speechOrigin('https://tts.example.com:8443/openai/v1'), 'https://tts.example.com:8443')
  assert.equal(speechOrigin('https://TTS.Example.com/v1'), 'https://tts.example.com')
  assert.equal(speechOrigin('https://[2001:db8::1]/v1'), 'https://[2001:db8::1]')
})

test('plain http only for hosts on this computer or the user’s own network', () => {
  for (const url of [
    'http://localhost:8880/v1',
    'http://127.0.0.1:8880/v1',
    'http://192.168.1.20:5002/v1',
    'http://10.0.0.5/v1',
    'http://172.16.4.2/v1',
    'http://kokoro.local:8880/v1',
    'http://tts.home.arpa/v1',
    'http://tts:8880/v1',
    'http://[::1]:8880/v1',
  ])
    assert.ok(speechOrigin(url), url)
  for (const url of ['http://api.openai.com/v1', 'http://8.8.8.8/v1', 'http://172.32.0.1/v1'])
    assert.equal(speechOrigin(url), '', url)
})

test('nothing that could widen or inject into the Content-Security-Policy passes', () => {
  for (const url of [
    '',
    'https:',
    'https://',
    'https:/',
    '*',
    'https://*',
    'https://*.example.com/v1',
    'https://example.com/v1; script-src *',
    "https://example.com/v1'unsafe-eval'",
    'https://example.com/v1 https:',
    'https://example.com,https://evil.example/v1',
    'https://user:pw@example.com/v1',
    'https://example.com/v1?key=1',
    'https://example.com/v1#x',
    'javascript:alert(1)',
    'data:text/plain,hi',
    'ws://localhost/v1',
    'ftp://example.com',
    `https://example.com/${'a'.repeat(600)}`,
    'https://exa mple.com/v1',
    'https://ex\\ample.com/v1',
  ])
    assert.equal(speechOrigin(url), '', JSON.stringify(url))
})

test('an unacceptable address clears the synced service; names are bounded', () => {
  assert.deepEqual(sanitizeSpeechService({ baseUrl: 'https:', model: 'tts-1', voice: 'alloy' }), {
    baseUrl: '',
    model: '',
    voice: '',
  })
  const ok = sanitizeSpeechService({ baseUrl: ' https://api.openai.com/v1/ ', model: ' tts-1 ', voice: 'alloy', apiKey: 'sk-x' })
  assert.deepEqual(ok, { baseUrl: 'https://api.openai.com/v1', model: 'tts-1', voice: 'alloy' })
  assert.equal(speechConfigured(ok), true)
  assert.equal(speechConfigured(sanitizeSpeechService({ baseUrl: 'https://a.example', model: 'm', voice: 'v\n' })), false)
})

test('preferences sync the address, model and voice but never a key', () => {
  assert.deepEqual(sanitizePreferences({}).speech, defaultPreferences.speech)
  const synced = sanitizePreferences({
    speech: { baseUrl: 'https://api.openai.com/v1', model: 'tts-1', voice: 'nova', apiKey: 'sk-secret', key: 'sk-secret' },
  })
  assert.deepEqual(synced.speech, { baseUrl: 'https://api.openai.com/v1', model: 'tts-1', voice: 'nova' })
  assert.ok(!JSON.stringify(synced).includes('sk-secret'))
})

test('the request carries the key only as a bearer token, without cookies, referrer or redirects', () => {
  const service = { baseUrl: 'https://api.openai.com/v1', model: 'tts-1', voice: 'alloy' }
  const { url, init } = speechRequest(service, 'sk-test', `hello ${'x'.repeat(2000)}`)
  assert.equal(url, 'https://api.openai.com/v1/audio/speech')
  assert.equal(init.method, 'POST')
  assert.equal(init.headers.Authorization, 'Bearer sk-test')
  assert.equal(init.credentials, 'omit')
  assert.equal(init.redirect, 'error')
  assert.equal(init.referrerPolicy, 'no-referrer')
  const body = JSON.parse(init.body)
  assert.deepEqual(Object.keys(body).sort(), ['input', 'model', 'response_format', 'voice'])
  assert.equal(body.input.length, 1000)
  assert.ok(!init.body.includes('sk-test'))
  assert.equal(speechRequest(service, '', 'hi').init.headers.Authorization, undefined)
})

test('keys with spaces or too long are refused', () => {
  assert.equal(sanitizeSpeechKey(' sk-abc '), 'sk-abc')
  assert.equal(sanitizeSpeechKey('sk abc'), '')
  assert.equal(sanitizeSpeechKey('x'.repeat(501)), '')
})

test('a request that never got an answer names CORS', () => {
  assert.match(speechFailure(new TypeError('Failed to fetch')), /CORS/)
  assert.match(speechFailure(null, 401), /refused the API key/)
  assert.match(speechFailure(null, 404), /\/audio\/speech/)
})
