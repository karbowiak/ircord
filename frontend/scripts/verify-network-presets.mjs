import assert from 'node:assert/strict'
import { computed, effectScope, nextTick, reactive, ref, watch, watchEffect } from 'vue'
import { createNetworkForm } from '../app/composables/network/useNetworkForm.ts'
import { networkPresets, normalizeNetworkHost } from '../shared/network-presets.ts'
import { createIRCordClient } from '../shared/api/client.ts'

const originals = new Map()
function replaceGlobal(name, value) {
  if (!originals.has(name)) originals.set(name, Object.getOwnPropertyDescriptor(globalThis, name))
  Object.defineProperty(globalThis, name, { configurable: true, writable: true, value })
}
function deferred() {
  let resolve, reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
const settle = async () => { await Promise.resolve(); await nextTick(); await Promise.resolve() }
const operator = {
  name: 'Operator network', host: 'irc.example.test', port: 6697, tls: true,
  authMethod: 'server-password', autoJoin: ['#welcome'], color: 'mint', initial: 'X',
  passwordProvided: true,
}
const legacy = [{ name: 'Runtime network', host: 'runtime.example.test', passwordProvided: true }]

function fixture({ props = {}, existing = [], runtime = legacy } = {}) {
  const mounted = []
  const calls = []
  const submissions = []
  const auth = reactive({ status: 'signed-in', account: { id: 'account-a' } })
  const globals = {
    computed, ref, reactive, watch, watchEffect, nextTick,
    onMounted: (callback) => mounted.push(callback),
    provide: () => {},
    useI18n: () => ({ t: (key) => key }),
    useChatStore: () => ({
      data: { networks: existing, user: { nick: 'tester' } },
      addNetwork: { isLoading: false, mutateAsync: async (input) => { submissions.push(input); return { network: { id: 'fixture' } } } },
      updateNetwork: { isLoading: false, mutateAsync: async ({ input }) => { submissions.push(input); return { id: 'fixture' } } },
    }),
    useAuthStore: () => auth,
    usePreferencesStore: () => ({ preferences: { networkIcons: {} } }),
    usePrompt: () => ({ confirm: async () => false }),
    useRuntimeConfig: () => ({ public: { networkPresets: runtime } }),
    apiClient: () => ({
      networkPresets(signal) {
        const pending = deferred()
        calls.push({ ...pending, signal })
        return pending.promise
      },
      checkNetwork: async () => ({ reachable: true, detail: '' }),
    }),
    errorText: () => 'offline',
  }
  for (const [name, value] of Object.entries(globals)) replaceGlobal(name, value)
  const scope = effectScope()
  const form = scope.run(() => {
    const value = createNetworkForm({ networkId: '', prefill: {}, firstConnection: false, ...props }, () => {})
    for (const callback of mounted) callback()
    return value
  })
  return { form, calls, auth, submissions, stop: () => scope.stop() }
}

try {
  {
    let request
    const client = createIRCordClient('http://127.0.0.1:1/api/v1', {
      fetch: async (value) => { request = value; return Response.json({ configured: true, items: [operator] }) },
    })
    const result = await client.networkPresets()
    assert.equal(new URL(request.url).pathname, '/api/v1/networks/presets')
    assert.equal(request.credentials, 'include')
    assert.equal(result.items[0].name, operator.name)
  }
  {
    const f = fixture({ props: { firstConnection: true } })
    assert.deepEqual(f.form.presets.value, [], 'fallback is not shown while loading')
    assert.equal(f.form.form.host, '', 'fallback does not prefill while loading')
    f.calls[0].resolve({ configured: true, items: [operator] })
    await settle()
    assert.equal(f.form.form.host, operator.host)
    assert.equal(f.form.presets.value[0].initial, 'X')
    assert.equal(f.form.providedPassword.value, true)
    assert.equal(f.form.advanced.value, false, 'a complete operator preset keeps quick setup compact')
    f.form.form.host = 'IRC.EXAMPLE.TEST'
    assert.equal(f.form.providedPassword.value, true, 'host matching ignores case')
    f.form.form.port = 6667
    assert.equal(f.form.providedPassword.value, false, 'port must match')
    f.form.form.port = 6697
    f.form.form.tls = false
    assert.equal(f.form.providedPassword.value, false, 'TLS must match')
    f.form.form.tls = true
    f.form.form.method = 'none'
    assert.equal(f.form.providedPassword.value, false, 'auth method must match')
    assert.equal(f.calls.length, 1, 'typing does not refetch')
    f.stop()
  }
  for (const outcome of ['unconfigured', 'error']) {
    const f = fixture({ props: { firstConnection: true } })
    if (outcome === 'error') f.calls[0].reject(new Error('offline'))
    else f.calls[0].resolve({ configured: false, items: [operator] })
    await settle()
    assert.equal(f.form.form.host, 'runtime.example.test', outcome)
    assert.equal(f.form.providedPassword.value, false, 'non-server-password presets cannot claim a password')
    f.stop()
  }
  for (const outcome of ['unconfigured', 'error']) {
    const runtime = [{
      name: 'Runtime password', host: 'RUNTIME.EXAMPLE.TEST.', port: 6697, tls: true,
      authMethod: 'server-password', passwordProvided: true,
    }]
    const f = fixture({ props: { firstConnection: true }, runtime })
    if (outcome === 'error') f.calls[0].reject(new Error('offline'))
    else f.calls[0].resolve({ configured: false, items: [] })
    await settle()
    assert.equal(f.form.form.host, 'runtime.example.test', outcome)
    assert.equal(f.form.providedPassword.value, true, `${outcome}: legacy hint is retained`)
    f.form.form.host = 'RUNTIME.EXAMPLE.TEST.'
    assert.equal(f.form.providedPassword.value, true, 'canonical host matches')
    f.form.form.host = 'other.example.test'
    assert.equal(f.form.providedPassword.value, false, 'host must match')
    f.form.form.host = 'runtime.example.test'
    f.form.form.port = 6667
    assert.equal(f.form.providedPassword.value, false, 'port must match')
    f.form.form.port = 6697
    f.form.form.tls = false
    assert.equal(f.form.providedPassword.value, false, 'TLS must match')
    f.form.form.tls = true
    f.form.form.method = 'nickserv'
    assert.equal(f.form.providedPassword.value, false, 'auth method must match')
    f.stop()
  }
  {
    const runtime = [{ ...operator, passwordProvided: true }]
    const f = fixture({ props: { firstConnection: true }, runtime })
    f.calls[0].resolve({ configured: true, items: [{ ...operator, passwordProvided: false }] })
    await settle()
    assert.equal(f.form.providedPassword.value, false, 'configured operator hint overrides legacy hint')
    f.stop()
  }
  {
    const runtime = [{ ...operator, passwordProvided: true }]
    const f = fixture({ props: { firstConnection: true }, runtime })
    f.calls[0].resolve({ configured: true, items: [] })
    await settle()
    assert.deepEqual(f.form.presets.value, [], 'configured empty list suppresses legacy hint')
    assert.equal(f.form.providedPassword.value, false)
    f.stop()
  }
  {
    const f = fixture({ props: { firstConnection: true } })
    f.calls[0].resolve({ configured: true, items: [] })
    await settle()
    assert.deepEqual(f.form.presets.value, [], 'configured empty list is authoritative')
    assert.equal(f.form.form.host, '')
    f.stop()
  }
  {
    const f = fixture({ props: { firstConnection: true } })
    f.form.form.host = 'typed.example.test'
    f.form.form.name = 'My server'
    f.calls[0].resolve({ configured: true, items: [operator] })
    await settle()
    assert.equal(f.form.form.host, 'typed.example.test')
    assert.equal(f.form.form.name, 'My server')
    assert.equal(f.form.presets.value[0].name, operator.name)
    f.stop()
  }
  {
    const f = fixture({ props: { firstConnection: true, prefill: { host: 'prefill.example.test' } } })
    f.calls[0].resolve({ configured: true, items: [operator] })
    await settle()
    assert.equal(f.form.form.host, 'prefill.example.test')
    assert.equal(f.form.form.name, '')
    f.stop()
  }
  {
    const existing = [{ id: 'saved', name: 'Saved', host: 'saved.example.test', port: 6697, tls: true,
      nick: 'nick', auth: { method: 'none' }, autoJoin: [], perform: [] }]
    const f = fixture({ props: { networkId: 'saved', firstConnection: true }, existing })
    f.calls[0].resolve({ configured: true, items: [operator] })
    await settle()
    assert.equal(f.form.form.host, 'saved.example.test')
    assert.equal(f.form.form.name, 'Saved')
    assert.equal(f.form.providedPassword.value, false, 'operator password does not alter existing edits')
    f.stop()
  }
  {
    const f = fixture({ props: { firstConnection: true } })
    f.auth.account = { id: 'account-b' }
    assert.equal(f.calls.length, 2)
    assert.equal(f.calls[0].signal.aborted, true)
    f.calls[0].resolve({ configured: true, items: [operator] })
    await settle()
    assert.deepEqual(f.form.presets.value, [], 'old account response cannot populate new account')
    const other = { ...operator, name: 'Other', host: 'other.example.test', initial: 'O' }
    f.calls[1].resolve({ configured: true, items: [other] })
    await settle()
    assert.equal(f.form.form.host, other.host)
    f.stop()
  }
  {
    const f = fixture({ props: { firstConnection: true } })
    f.calls[0].resolve({ configured: true, items: [operator] })
    await settle()
    assert.equal(f.form.form.host, operator.host)
    f.auth.account = { id: 'account-b' }
    assert.equal(f.form.form.host, '', 'previous account default is cleared while loading')
    const other = { ...operator, host: 'other.example.test' }
    f.calls[1].resolve({ configured: true, items: [other] })
    await settle()
    assert.equal(f.form.form.host, other.host)
    f.stop()
  }
  assert.equal(networkPresets([{ ...operator, tls: false }], true)[0].passwordProvided, false)
  assert.equal(networkPresets([{ ...operator, authMethod: 'nickserv' }], true)[0].passwordProvided, false)
  assert.equal(networkPresets([{ ...operator, tls: false }])[0].passwordProvided, false)
  assert.equal(networkPresets([{ ...operator, authMethod: 'nickserv' }])[0].passwordProvided, false)
  const legacyDefaultTLS = { ...operator }
  delete legacyDefaultTLS.tls
  assert.equal(networkPresets([legacyDefaultTLS])[0].passwordProvided, true, 'legacy TLS defaults to enabled')
  const manyChannels = Array.from({ length: 100 }, (_, i) => `#channel${i}`)
  assert.equal(networkPresets([{ ...operator, autoJoin: manyChannels }], true)[0].autoJoin.length, 100)
  assert.equal(networkPresets([{ ...operator, autoJoin: manyChannels }])[0].autoJoin.length, 70)
  for (const [input, expected] of [
    ['IRC.EXAMPLE.TEST.', 'irc.example.test'],
    ['127.0.0.1', '127.0.0.1'],
    ['123', '123'],
    ['2001:0DB8:0:0:0:0:0:1', '2001:db8::1'],
    ['[2001:db8::1]', '2001:db8::1'],
    ['::ffff:192.0.2.1', '192.0.2.1'],
  ]) assert.equal(normalizeNetworkHost(input), expected, input)
  for (const input of ['https://example.test', 'user@example.test', 'host/path', 'bad host', '[::1', ':::1', '127.0.0.999', '-bad.example'])
    assert.equal(normalizeNetworkHost(input), null, input)
  {
    const f = fixture()
    f.form.form.host = ':::1'
    assert.equal(f.form.hostProblem.value, 'networkForm.invalidHostname')
    f.form.form.host = '[2001:db8::1]'
    assert.equal(f.form.hostProblem.value, '')
    f.stop()
  }
  {
    const ipv6 = { ...operator, name: 'N'.repeat(64), host: '2001:db8::1' }
    const f = fixture({ props: { firstConnection: true } })
    f.calls[0].resolve({ configured: true, items: [ipv6] })
    await settle()
    assert.equal(f.form.form.name, ipv6.name)
    f.form.form.host = '[2001:0db8:0:0:0:0:0:1]'
    assert.equal(f.form.providedPassword.value, true, 'equivalent IPv6 endpoints match')
    f.form.form.nick = 'tester'
    f.form.verification.value = { state: 'verified', detail: '' }
    await f.form.submit()
    assert.equal(f.submissions[0].host, '2001:db8::1', 'brackets are removed on submission')
    f.stop()
  }
  console.log('Network presets: configured, fallback, empty, edits, account isolation and password matching passed.')
} finally {
  for (const [name, descriptor] of originals) {
    if (descriptor) Object.defineProperty(globalThis, name, descriptor)
    else delete globalThis[name]
  }
}
