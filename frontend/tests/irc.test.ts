import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  channelRights,
  markdownToIrc,
  parseFormatting,
  parsePrefix,
  stripFormatting,
  topPrefix,
  validRawLine,
} from '../shared/utils/irc'

const network = { isupport: { PREFIX: '(qaohv)~&@%+' } }
const channel = (modes: Record<string, string>, channelModes = 'nt') => ({
  kind: 'channel' as const,
  memberModes: modes,
  modes: channelModes,
})

test('PREFIX parsing falls back to the common default', () => {
  assert.deepEqual(parsePrefix({ PREFIX: '(ov)@+' }), { modes: 'ov', prefixes: '@+' })
  assert.deepEqual(parsePrefix({ PREFIX: 'garbage' }), { modes: 'qaohv', prefixes: '~&@%+' })
  assert.equal(topPrefix('a', channel({ a: '+@' }), network), '@')
})

test('channel rights follow prefixes and +t', () => {
  const c = channel({ op: '@', half: '%', voiced: '+' })
  assert.deepEqual(channelRights('op', c, network), {
    kick: true,
    ban: true,
    voice: true,
    op: true,
    invite: true,
    topic: true,
  })
  assert.equal(channelRights('half', c, network).op, false)
  assert.equal(channelRights('half', c, network).kick, true)
  assert.equal(channelRights('voiced', c, network).kick, false)
  assert.equal(channelRights('voiced', c, network).topic, false)
  assert.equal(channelRights('voiced', channel({ voiced: '+' }, 'n'), network).topic, true)
  // Owners outrank ops on networks that have them.
  assert.equal(channelRights('owner', channel({ owner: '~' }), network).op, true)
  // A network without halfops still lets ops kick.
  assert.equal(
    channelRights('op', channel({ op: '@' }), { isupport: { PREFIX: '(ov)@+' } }).kick,
    true,
  )
})

test('formatting codes become styled runs and strip cleanly', () => {
  const runs = parseFormatting('a\x02b\x02 \x0304,01red\x03 \x1ditalic\x0f plain')
  assert.deepEqual(
    runs.map((r) => [r.text, !!r.bold, r.fg ?? '', r.bg ?? '', !!r.italic]),
    [
      ['a', false, '', '', false],
      ['b', true, '', '', false],
      [' ', false, '', '', false],
      ['red', false, '#ff0000', '#000000', false],
      [' ', false, '', '', false],
      ['italic', false, '', '', true],
      [' plain', false, '', '', false],
    ],
  )
  assert.equal(stripFormatting('\x02bold\x02 \x0312blue\x03 \x04ff00ffhex'), 'bold blue hex')
  assert.deepEqual(parseFormatting('no codes'), [{ text: 'no codes' }])
})

test('composer markdown becomes IRC codes without touching URLs or snake_case', () => {
  assert.equal(
    markdownToIrc('**bold** and *it* and _it_'),
    '\x02bold\x02 and \x1dit\x1d and \x1dit\x1d',
  )
  assert.equal(
    markdownToIrc('`code` __under__ ~~gone~~'),
    '\x11code\x11 \x1funder\x1f \x1egone\x1e',
  )
  assert.equal(
    markdownToIrc('see https://x.test/a_b_c and snake_case_name'),
    'see https://x.test/a_b_c and snake_case_name',
  )
  assert.equal(markdownToIrc('2 * 3 * 4'), '2 * 3 * 4')
})

test('raw lines are single, bounded IRC lines', () => {
  assert.equal(validRawLine('PRIVMSG #a :hi'), true)
  assert.equal(validRawLine('PRIVMSG #a :hi\r\nQUIT'), false)
  assert.equal(validRawLine('x'.repeat(511)), false)
  assert.equal(validRawLine('   '), false)
})
