import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import {
  markdownToIrc,
  ircToMarkdown,
  parseFormatting,
  splitCodeBlocks,
  stripFormatting,
  textOutsideCode,
} from '../shared/utils/irc'

const fenced = [
  'before',
  '```py',
  'def f(a_b, *args):',
  '    return `x` ** 2  # ~~not strike~~ _not italic_ :smile: @alice',
  '',
  '\tindented with a tab   ',
  '```',
  'after',
].join('\n')

test('fenced code blocks split out with their language, whitespace and punctuation', () => {
  assert.deepEqual(splitCodeBlocks(fenced), [
    { kind: 'text', raw: 'before\n' },
    {
      kind: 'code',
      raw: fenced.slice('before\n'.length, fenced.length - '\nafter'.length),
      lang: 'py',
      code: [
        'def f(a_b, *args):',
        '    return `x` ** 2  # ~~not strike~~ _not italic_ :smile: @alice',
        '',
        '\tindented with a tab   ',
      ].join('\n'),
    },
    { kind: 'text', raw: '\nafter' },
  ])
  // Discord's one-line form, and a block with no language.
  assert.deepEqual(splitCodeBlocks('```a  b```'), [
    { kind: 'code', raw: '```a  b```', code: 'a  b' },
  ])
  assert.deepEqual(splitCodeBlocks('```\n  x\n```'), [
    { kind: 'code', raw: '```\n  x\n```', code: '  x' },
  ])
  // Two blocks in one message.
  assert.deepEqual(
    splitCodeBlocks('```\na\n```\nmid\n```\nb\n```').map((s) => s.kind),
    ['code', 'text', 'code'],
  )
})

test('malformed fences stay ordinary text', () => {
  for (const text of [
    '```\nnever closed',
    'inline ```not a fence```',
    '``````',
    '```\n```',
    'a ` b',
  ])
    assert.deepEqual(splitCodeBlocks(text), [{ kind: 'text', raw: text }])
})

test('the composer sends code verbatim and converts formatting only outside it', () => {
  // A fence travels exactly as typed: other IRC clients read plain Markdown.
  assert.equal(markdownToIrc(fenced), fenced)
  // Inline code is IRC monospace, and nothing inside it is formatted.
  assert.equal(markdownToIrc('`a**b**c` and `_x_`'), '\x11a**b**c\x11 and \x11_x_\x11')
  // Emphasis may still wrap code and URLs.
  assert.equal(markdownToIrc('**see `x` now**'), '\x02see \x11x\x11 now\x02')
  // A closing marker touching a URL belongs to the URL; a space ends the URL first.
  assert.equal(markdownToIrc('**https://x.test/a**'), '**https://x.test/a**')
  assert.equal(markdownToIrc('**see https://x.test/a **'), '\x02see https://x.test/a \x02')
  assert.equal(
    markdownToIrc('_see https://x.test/a_b_ now_'),
    '\x1dsee https://x.test/a_b_ now\x1d',
  )
  // Underscores in identifiers and URLs stay intact, including irc:// links.
  assert.equal(
    markdownToIrc('my_var and irc://irc.test/#a_b_c and _it_'),
    'my_var and irc://irc.test/#a_b_c and \x1dit\x1d',
  )
  // Unterminated code stays literal text, and the rest is still formatted.
  assert.equal(markdownToIrc('```\n**x**'), '```\n\x02x\x02')
  assert.equal(markdownToIrc('a `b'), 'a `b')
  // Unicode and existing IRC codes pass through.
  assert.equal(markdownToIrc('`🐈 ü` \x0304red\x03'), '\x11🐈 ü\x11 \x0304red\x03')
  // A NUL can never collide with the internal placeholders.
  assert.equal(markdownToIrc('\x000\x00 `x`'), '0 \x11x\x11')
})

test('code is excluded from link and emoji detection', () => {
  const outside = textOutsideCode(
    'a \x11https://in.test/x\x11\n```\nhttps://in.test/y\n```\nhttps://out.test/z',
  )
  assert.deepEqual(outside.match(/https:\S+/g), ['https://out.test/z'])
  // Native IRC formatting outside code is preserved.
  assert.deepEqual(
    parseFormatting(markdownToIrc('**b** `c`')).map((r) => [r.text, !!r.bold, !!r.mono]),
    [
      ['b', true, false],
      [' ', false, false],
      ['c', false, true],
    ],
  )
  assert.equal(stripFormatting(markdownToIrc('**b** `c`')), 'b c')
})

test('splitting stays linear on large and hostile input', () => {
  const started = performance.now()
  splitCodeBlocks('```\n'.repeat(50_000))
  splitCodeBlocks('x```'.repeat(50_000))
  markdownToIrc('`'.repeat(4000) + '*_'.repeat(2000))
  assert.ok(performance.now() - started < 1000)
})

test('the browser and the server agree on which links are outside code', () => {
  // The Go test (backend/internal/irc/formatting_test.go) loads the same table.
  const cases = JSON.parse(
    readFileSync(new URL('./fixtures/code-parity.json', import.meta.url), 'utf8'),
  ) as { name: string; text: string; urls: string[] }[]
  assert.ok(cases.length > 0)
  for (const { name, text, urls } of cases)
    assert.deepEqual(textOutsideCode(text).match(/https:\/\/[^\s<>]+/g) ?? [], urls, name)
})

test('an edit starts from the Markdown that produced the IRC formatting', () => {
  for (const typed of [
    'plain text',
    '**bold** and *italic* and __under__ and ~~gone~~',
    'code `x = 1` here',
    '```\nfenced **kept**\n```',
  ])
    assert.equal(markdownToIrc(ircToMarkdown(markdownToIrc(typed))), markdownToIrc(typed))
  assert.equal(ircToMarkdown('\x02bold\x02 \x0304red\x03'), '**bold** red')
})
