import assert from 'node:assert/strict'
import test from 'node:test'
import { commandAliases, commands, type CommandSpec } from './commands'
import {
  commandArgumentsValid,
  exactParameterizedCommand,
  resolveCommand,
  serializeCommandArguments,
} from './command-arguments'

const command = (name: string) => {
  const result = resolveCommand(name)
  assert.ok(result, name)
  return result
}

test('catalog covers every existing command and its guided fields', () => {
  const expected: Record<string, string[]> = {
    me: ['action'],
    slap: ['nickname'],
    msg: ['nickname', 'message'],
    query: ['nickname'],
    join: ['channel', 'key'],
    part: ['reason'],
    nick: ['nickname'],
    topic: ['topic'],
    away: ['message'],
    status: ['emoji', 'message', 'duration'],
    afk: ['reason'],
    brb: ['reason'],
    back: [],
    op: ['nickname'],
    deop: ['nickname'],
    voice: ['nickname'],
    devoice: ['nickname'],
    kick: ['nickname', 'reason'],
    ban: ['mask'],
    invite: ['nickname', 'channel'],
    mode: ['modes', 'arguments'],
    whois: ['nickname'],
    ignore: ['mask'],
    unignore: ['mask'],
    list: ['filter'],
    server: [],
    settings: [],
    banlist: [],
    gif: ['query'],
    quote: ['line'],
    attract: [],
  }
  assert.deepEqual(
    Object.fromEntries(commands.map((spec) => [spec.name, spec.fields.map((f) => f.name)])),
    expected,
  )
  for (const spec of commands) {
    assert.equal(Boolean(spec.args), Boolean(spec.fields.length), spec.name)
    for (const field of spec.fields) {
      assert.ok(field.label.startsWith('workspaceUi.commandEditor.fields.'))
      assert.ok(field.hint.startsWith('workspaceUi.commandEditor.hints.'))
    }
  }
  assert.equal(command('attract').operator, true)
  assert.equal(command('attract').hidden, true)
  assert.equal(command('settings').channel, true)
  assert.equal(command('nick').fields[0]?.type, 'text')
  assert.equal(command('nick').fields[0]?.autocomplete, undefined)
})

test('resolves aliases, case and exact parameterized prefixes only', () => {
  for (const [alias, canonical] of Object.entries(commandAliases)) {
    assert.equal(resolveCommand(`/${alias.toUpperCase()}`), command(canonical))
    assert.equal(exactParameterizedCommand(`/${alias} `), command(canonical))
  }
  for (const input of ['/unknown', '/back', '//msg', '/msg hi', '/msg  ', '/msg\n'])
    assert.equal(exactParameterizedCommand(input), undefined, input)
  assert.equal(resolveCommand('unknown'), undefined)
  assert.equal(exactParameterizedCommand('/msg'), command('msg'))
})

test('validates required and optional fields and serializes optional queries', () => {
  assert.deepEqual(commandArgumentsValid(command('msg'), { nickname: 'Sam' }), {
    field: 'message',
    error: 'required',
  })
  assert.deepEqual(commandArgumentsValid(command('query'), {}), {
    field: 'nickname',
    error: 'required',
  })
  for (const name of ['topic', 'ignore', 'away', 'part', 'list', 'afk', 'brb', 'back']) {
    assert.equal(commandArgumentsValid(command(name), {}), null, name)
    assert.equal(serializeCommandArguments(command(name), {}), `/${name}`)
  }
  assert.equal(serializeCommandArguments(command('invite'), { nickname: 'Sam' }), '/invite Sam')
  assert.equal(
    serializeCommandArguments(command('invite'), { nickname: 'Sam', channel: '#one' }),
    '/invite Sam #one',
  )
  assert.equal(
    serializeCommandArguments(command('join'), { channel: '#one', key: 'key' }),
    '/join #one key',
  )
  assert.equal(
    serializeCommandArguments(command('mode'), { modes: '+o', arguments: 'Sam' }),
    '/mode +o Sam',
  )
})

test('preserves multiword action, message, and reason text and the message recipient', () => {
  assert.equal(
    serializeCommandArguments(command('me'), { action: 'waves  at everyone' }),
    '/me waves  at everyone',
  )
  assert.equal(
    serializeCommandArguments(command('msg'), { nickname: 'Sam', message: 'Hello  there' }),
    '/msg Sam Hello  there',
  )
  assert.equal(
    serializeCommandArguments(command('kick'), { nickname: 'Sam', reason: 'too  loud' }),
    '/kick Sam too  loud',
  )
})

test('status accepts empty and emoji-only values but requires its domain adapter', () => {
  const status = command('status')
  assert.equal(commandArgumentsValid(status, {}), null)
  assert.equal(commandArgumentsValid(status, { emoji: '☕' }), null)
  assert.equal(commandArgumentsValid(status, { message: 'Busy', duration: 'today' }), null)
  assert.throws(() => serializeCommandArguments(status, {}), /domain adapter/)
  assert.throws(() => serializeCommandArguments(status, { emoji: '☕' }), /domain adapter/)
  assert.throws(
    () => serializeCommandArguments(status, { message: 'Busy', duration: 'today' }),
    /domain adapter/,
  )
})

test('rejects controls, token whitespace and byte overflow without exposing values', () => {
  for (const value of [
    'x\rINJECT',
    'x\nINJECT',
    'x\0INJECT',
    'x\tINJECT',
    'x\u007fINJECT',
    'x\ud800',
  ]) {
    assert.deepEqual(commandArgumentsValid(command('quote'), { line: value }), {
      field: 'line',
      error: 'control',
    })
    assert.throws(
      () => serializeCommandArguments(command('quote'), { line: value }),
      (error: Error) => !error.message.includes(value),
    )
  }
  assert.equal(command('join').fields[1]?.type, 'secret')
  assert.deepEqual(commandArgumentsValid(command('join'), { channel: '#c', key: 'private key' }), {
    field: 'key',
    error: 'token',
  })
  assert.throws(
    () => serializeCommandArguments(command('join'), { channel: '#c', key: 'private key' }),
    (error: Error) => !error.message.includes('private key'),
  )
  assert.equal(commandArgumentsValid(command('status'), { message: 'é'.repeat(100) }), null)
  assert.deepEqual(commandArgumentsValid(command('status'), { message: 'é'.repeat(101) }), {
    field: 'message',
    error: 'maxBytes',
  })
  assert.deepEqual(
    commandArgumentsValid(command('status'), { message: 'ok', emoji: 'é'.repeat(33) }),
    { field: 'emoji', error: 'maxBytes' },
  )
})

test('rejects gaps and invalid choices in reusable schemas', () => {
  const spec: CommandSpec = {
    name: 'example',
    args: '[one] [two]',
    description: '',
    fields: [
      { name: 'one', type: 'text', label: '', hint: '' },
      { name: 'two', type: 'text', label: '', hint: '', choices: [{ value: 'a', label: 'A' }] },
    ],
  }
  assert.deepEqual(commandArgumentsValid(spec, { two: 'b' }), { field: 'two', error: 'choice' })
  assert.throws(() => serializeCommandArguments(spec, { two: 'a' }), /order/)
})
