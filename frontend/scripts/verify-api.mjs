import assert from 'node:assert/strict'
import { once } from 'node:events'
import WebSocket from 'ws'

const base = process.env.TEST_BASE_URL || 'http://127.0.0.1:3000'
const connect = async (cookie) => {
  const ws = new WebSocket(base.replace(/^http/, 'ws') + '/api/v1/events', {
    headers: { cookie, origin: base },
  })
  const events = []
  ws.on('message', (bytes) => events.push(JSON.parse(bytes.toString())))
  await once(ws, 'open')
  return { ws, events }
}
const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
const until = async (predicate, message) => {
  const started = Date.now()
  while (!predicate()) {
    if (Date.now() - started > 6000) throw Error(message)
    await pause(15)
  }
}
/** Registers a fresh key account and loads its workspace. */
async function session() {
  const registered = await fetch(base + '/api/v1/auth/register', { method: 'POST' })
  assert.equal(registered.status, 200)
  const { key } = await registered.json()
  assert.match(key, /^ircord-(?:[0-9A-Z]{5}-){5}[0-9A-Z]{5}$/)
  const cookie = registered.headers.get('set-cookie').split(';')[0]
  const r = await fetch(base + '/api/v1/bootstrap', { headers: { cookie } })
  assert.equal(r.status, 200)
  return { cookie, key, data: await r.json() }
}
// Signed out: chat routes refuse, the session check answers with no account.
assert.equal((await fetch(base + '/api/v1/bootstrap')).status, 401)
assert.equal((await fetch(base + '/api/app/library')).status, 401)
assert.deepEqual(await (await fetch(base + '/api/v1/auth/session')).json(), { account: null })
const a = await session(),
  b = await session()
assert.notEqual(a.cookie, b.cookie)
const sockets = []
try {
  const first = await connect(a.cookie),
    sameUser = await connect(a.cookie),
    other = await connect(b.cookie)
  sockets.push(first.ws, sameUser.ws, other.ws)
  const text = 'isolated-' + crypto.randomUUID(),
    clientId = crypto.randomUUID()
  const command = { v: 1, type: 'message.send', conversationId: 'general', clientId, text }
  first.ws.send(JSON.stringify(command))
  await until(
    () =>
      first.events.filter((e) => e.type === 'message.created' && e.data.text === text).length === 2,
    'Did not receive acknowledgement and echo',
  )
  await until(
    () =>
      sameUser.events.filter((e) => e.type === 'message.created' && e.data.text === text).length ===
      2,
    'Second device did not receive messages',
  )
  assert.equal(
    other.events.some((e) => e.data?.text === text),
    false,
    'Cross-session event leak',
  )
  const before = await fetch(base + '/api/v1/conversations/general/messages', {
    headers: { cookie: a.cookie },
  }).then((r) => r.json())
  first.ws.send(JSON.stringify(command))
  await pause(650)
  const after = await fetch(base + '/api/v1/conversations/general/messages', {
    headers: { cookie: a.cookie },
  }).then((r) => r.json())
  assert.equal(after.items.length, before.items.length, 'Retry produced duplicates')
  const privateHistory = await fetch(base + '/api/v1/conversations/general/messages', {
    headers: { cookie: b.cookie },
  }).then((r) => r.json())
  assert.equal(
    privateHistory.items.some((m) => m.text === text),
    false,
    'Cross-session history leak',
  )
  sameUser.events.length = 0
  sameUser.ws.send(JSON.stringify({ v: 1, type: 'session.resume', after: 0 }))
  await until(
    () =>
      sameUser.events.filter((e) => e.type === 'message.created' && e.data.text === text).length ===
      2,
    'Replay did not restore missed messages',
  )
  first.ws.send('{malformed')
  await until(
    () => first.events.some((e) => e.type === 'command.error'),
    'Malformed frame was not handled',
  )
  const avatarDataUrl =
    'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aF10AAAAASUVORK5CYII='
  const profileResponse = await fetch(base + '/api/v1/profile', {
    method: 'PATCH',
    headers: { cookie: a.cookie, 'content-type': 'application/json' },
    body: JSON.stringify({
      displayName: 'Avatar owner',
      bio: 'A private profile',
      nameColor: '#ee88cc',
      avatarDataUrl,
    }),
  })
  assert.equal(profileResponse.status, 200)
  const profile = await profileResponse.json()
  assert.equal(profile.nameColor, '#ee88cc')
  const avatarResponse = await fetch(base + profile.avatarUrl, { headers: { cookie: a.cookie } })
  assert.equal(avatarResponse.status, 200)
  assert.equal(avatarResponse.headers.get('content-type'), 'image/png')
  assert.ok((await avatarResponse.arrayBuffer()).byteLength > 0)
  assert.match(avatarResponse.headers.get('cache-control'), /private/)
  const wrongSessionAvatar = await fetch(base + profile.avatarUrl, {
    headers: { cookie: b.cookie },
  })
  assert.equal(wrongSessionAvatar.status, 404, 'Cross-session avatar leak')
  await until(
    () => sameUser.events.some((e) => e.type === 'profile.updated'),
    'Profile update not delivered',
  )
  assert.equal(
    JSON.stringify(sameUser.events.filter((e) => e.type === 'profile.updated')).includes('base64'),
    false,
    'Image data must not be embedded in replay',
  )
  const bad = await fetch(base + '/api/v1/conversations/general/messages?before=missing', {
    headers: { cookie: a.cookie },
  })
  assert.equal(bad.status, 400)
  const isolatedConversation = await fetch(base + '/api/v1/conversations', {
    method: 'POST',
    headers: { cookie: a.cookie, 'content-type': 'application/json' },
    body: JSON.stringify({ networkId: 'libera', name: 'private-' + Date.now() }),
  }).then((r) => r.json())
  const forbidden = await fetch(
    base + `/api/v1/conversations/${isolatedConversation.id}/messages`,
    { headers: { cookie: b.cookie } },
  )
  assert.equal(forbidden.status, 404)
  await new Promise((resolve, reject) => {
    const socket = new WebSocket(base.replace(/^http/, 'ws') + '/api/v1/events', {
      headers: { cookie: a.cookie, origin: 'https://not-ircord.example' },
    })
    socket.on('open', () => {
      socket.close()
      reject(Error('Cross-origin socket accepted'))
    })
    socket.on('unexpected-response', (_, r) => {
      assert.equal(r.statusCode, 403)
      resolve()
    })
    socket.on('error', () => {})
  })
  console.log(
    'PASS: real WebSocket echo, device fanout, session isolation, HTTP history isolation, idempotency, replay, malformed frames, cursor validation, conversation authorization, and origin rejection.',
  )
} finally {
  for (const ws of sockets) ws.close()
}

// Frontend-owned Nitro routes: appearance and picker library, keyed by the account cookie.
async function account() {
  const { cookie } = await session()
  const r = await fetch(base + '/api/app/library', { headers: { cookie } })
  assert.equal(r.status, 200)
  assert.match(r.headers.get('cache-control') ?? '', /no-store/)
  return cookie
}
const app = (cookie, path, init = {}) =>
  fetch(base + '/api/app' + path, {
    ...init,
    headers: { cookie, 'content-type': 'application/json', ...init.headers },
  })
const one = await account(),
  two = await account()
assert.notEqual(one, two)
let r = await app(one, '/appearance', {
  method: 'PUT',
  body: JSON.stringify({
    settings: { schemeDark: 'nord-dark', font: '"><script>', messageSize: 99 },
  }),
})
assert.equal(r.status, 200)
const saved = await r.json()
assert.equal(saved.settings.schemeDark, 'nord-dark')
assert.equal(saved.settings.font, 'geist')
assert.equal(saved.settings.messageSize, 20)
assert.equal((await (await app(two, '/appearance')).json()).settings, null)
r = await app(one, '/appearance', { method: 'PUT', body: '{}' })
assert.equal(r.status, 400)

const gif = {
  id: 'verify-1',
  slug: 'verify-1',
  title: 'Verify',
  url: 'https://static.klipy.com/v.gif',
  previewUrl: 'https://static.klipy.com/v.webp',
  stillUrl: 'https://static.klipy.com/v.jpg',
  width: 100,
  height: 100,
  provider: 'klipy',
}
assert.equal(
  (await app(one, '/library/gifs', { method: 'POST', body: JSON.stringify(gif) })).status,
  200,
)
r = await app(one, '/library/gifs', {
  method: 'POST',
  body: JSON.stringify({ ...gif, id: 'x', url: 'https://evil.example/x.gif' }),
})
assert.equal(r.status, 400)
// Concurrent increments from two "devices" must both count.
await Promise.all(
  Array.from({ length: 10 }, () =>
    app(one, '/library', { method: 'PATCH', body: JSON.stringify({ uses: { '1F602': 1 } }) }),
  ),
)
const library = await (await app(one, '/library')).json()
assert.deepEqual(
  library.gifs.map((g) => g.id),
  ['verify-1'],
)
assert.equal(library.emojiUsage.find((u) => u.id === '1F602')?.count, 10)
assert.deepEqual((await (await app(two, '/library')).json()).gifs, [])
assert.equal((await app(one, `/library/gifs/${gif.id}`, { method: 'DELETE' })).status, 200)
console.log(
  'PASS: appearance and library are account-isolated, sanitized, and keep concurrent usage counts.',
)

// Key accounts: the key signs in from a fresh browser; rotating invalidates the old key.
{
  const { cookie, key } = await session()
  const login = await fetch(base + '/api/v1/auth/login', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ key: key.toLowerCase() }),
  })
  assert.equal(login.status, 200)
  const second = login.headers.get('set-cookie').split(';')[0]
  assert.notEqual(second, cookie)
  const rotated = await (
    await fetch(base + '/api/v1/auth/rotate', { method: 'POST', headers: { cookie } })
  ).json()
  const stale = await fetch(base + '/api/v1/auth/login', {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ key }),
  })
  assert.equal(stale.status, 401)
  assert.notEqual(rotated.key, key)

  // Raw commands, the server log and per-channel modes, over the socket.
  const { ws, events } = await connect(cookie)
  await until(() => events.some((e) => e.type === 'session.ready'), 'no ready event')
  ws.send(
    JSON.stringify({
      v: 1,
      type: 'raw.send',
      clientId: 'raw-1',
      networkId: 'libera',
      line: 'MODE #general +v ada',
    }),
  )
  await until(
    () => events.some((e) => e.type === 'conversation.updated' && e.data.memberModes?.ada === '+'),
    'mode change not applied',
  )
  ws.send(
    JSON.stringify({
      v: 1,
      type: 'raw.send',
      clientId: 'raw-2',
      networkId: 'libera',
      line: 'QUIT :bye',
    }),
  )
  await until(
    () => events.some((e) => e.type === 'command.error' && e.clientId === 'raw-2'),
    'QUIT was not refused',
  )
  ws.send(
    JSON.stringify({
      v: 1,
      type: 'raw.send',
      clientId: 'raw-3',
      networkId: 'libera',
      line: 'A\r\nQUIT',
    }),
  )
  await until(
    () => events.some((e) => e.type === 'command.error' && e.clientId === 'raw-3'),
    'CRLF was not refused',
  )
  const log = await (
    await fetch(base + '/api/v1/networks/libera/server-log', { headers: { cookie } })
  ).json()
  assert.ok(log.items.some((l) => l.kind === 'outgoing' && l.text === 'MODE #general +v ada'))
  ws.close()

  // Network passwords are write-only.
  const network = await (
    await fetch(base + '/api/v1/networks', {
      method: 'POST',
      headers: { cookie, 'content-type': 'application/json' },
      body: JSON.stringify({
        name: 'Secret',
        host: 'irc.example.org',
        port: 6697,
        tls: true,
        nick: 'tester',
        auth: { method: 'sasl-plain', account: 'tester', password: 'hunter2' },
        autoJoin: ['#a'],
        perform: [],
      }),
    })
  ).json()
  assert.deepEqual(network.auth, { method: 'sasl-plain', account: 'tester', passwordSet: true })
  assert.equal(JSON.stringify(network).includes('hunter2'), false)

  // Uploads are private to the account.
  const form = new FormData()
  form.append('file', new Blob(['hello'], { type: 'text/plain' }), 'hello.txt')
  const upload = await (
    await fetch(base + '/api/v1/uploads', { method: 'POST', headers: { cookie }, body: form })
  ).json()
  assert.equal((await fetch(base + upload.url, { headers: { cookie } })).status, 200)
  const other = await session()
  assert.equal((await fetch(base + upload.url, { headers: { cookie: other.cookie } })).status, 404)
  console.log(
    'PASS: key accounts, raw commands, modes, server log, write-only passwords and private uploads.',
  )
}

// Notification rules, push subscriptions, channel lists and network connection state.
{
  const { cookie } = await session()
  const json = { cookie, 'content-type': 'application/json' }
  const saved = await (
    await fetch(base + '/api/v1/notifications/settings', {
      method: 'PUT',
      headers: json,
      body: JSON.stringify({
        channelDefault: 'all',
        highlightWords: ['ircord', ''],
        ignore: ['troll'],
        levels: { general: 'nope' },
      }),
    })
  ).json()
  assert.equal(saved.channelDefault, 'all')
  assert.deepEqual(saved.highlightWords, ['ircord'])
  assert.deepEqual(saved.levels, {})
  const other = await session()
  const theirs = await (
    await fetch(base + '/api/v1/notifications/settings', { headers: { cookie: other.cookie } })
  ).json()
  assert.deepEqual(theirs.ignore, [])

  const key = await (await fetch(base + '/api/v1/push/key', { headers: { cookie } })).json()
  assert.match(key.publicKey, /^[\w-]{80,}$/)
  const bad = await fetch(base + '/api/v1/push/subscriptions', {
    method: 'POST',
    headers: json,
    body: JSON.stringify({
      subscription: { endpoint: 'https://evil.example/hook', keys: { p256dh: 'x', auth: 'y' } },
    }),
  })
  assert.equal(bad.status, 400)

  const { ws, events } = await connect(cookie)
  await until(() => events.some((e) => e.type === 'session.ready'), 'no ready event')
  ws.send(
    JSON.stringify({
      v: 1,
      type: 'raw.send',
      clientId: 'ban-1',
      networkId: 'libera',
      line: 'MODE #general +b troll!*@*',
    }),
  )
  await until(
    () => events.some((e) => e.type === 'message.created' && e.data.event?.text?.includes('+b')),
    'ban not applied',
  )
  const bans = await (
    await fetch(base + '/api/v1/conversations/general/lists/b', { headers: { cookie } })
  ).json()
  assert.ok(bans.items.some((b) => b.mask === 'troll!*@*'))
  assert.equal(
    (await fetch(base + '/api/v1/conversations/general/lists/z', { headers: { cookie } })).status,
    400,
  )
  ws.send(
    JSON.stringify({
      v: 1,
      type: 'raw.send',
      clientId: 'mode-1',
      networkId: 'libera',
      line: 'MODE #general +kl secret 25',
    }),
  )
  await until(
    () =>
      events.some(
        (e) =>
          e.type === 'conversation.updated' &&
          e.data.modeParams?.l === '25' &&
          e.data.modeParams?.k === 'secret',
      ),
    'key and limit not applied',
  )

  await fetch(base + '/api/v1/networks/oftc', {
    method: 'PATCH',
    headers: json,
    body: JSON.stringify({ status: 'disconnected' }),
  })
  const connecting = await (
    await fetch(base + '/api/v1/networks/oftc', {
      method: 'PATCH',
      headers: json,
      body: JSON.stringify({ status: 'connected' }),
    })
  ).json()
  assert.equal(connecting.status, 'connecting')
  await until(
    () =>
      events.some(
        (e) =>
          e.type === 'network.updated' && e.data.id === 'oftc' && e.data.status === 'connected',
      ),
    'network never finished connecting',
  )
  ws.close()
  console.log(
    'PASS: notification rules, push validation, channel lists, mode parameters and connection state.',
  )
}
