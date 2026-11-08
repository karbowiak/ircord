import { test } from 'node:test'
import assert from 'node:assert/strict'
import { messageBlocks, type MessageBlock } from '../shared/utils/blocks'
import { splitQuotes } from '../shared/utils/quotes'

const shapes = (text: string) =>
  splitQuotes(text).map((block) => (block.kind === 'text' ? block.text : { quote: block.text }))

test('lines starting with "> " become one quote, and the text around it stays text', () => {
  assert.deepEqual(shapes('> hello'), [{ quote: 'hello' }])
  assert.deepEqual(shapes('before\n> one\n> two\nafter'), [
    'before',
    { quote: 'one\ntwo' },
    'after',
  ])
  // A non-quote line splits two quotes.
  assert.deepEqual(shapes('> a\nb\n> c'), [{ quote: 'a' }, 'b', { quote: 'c' }])
  // An empty quoted line keeps the quote going.
  assert.deepEqual(shapes('> a\n> \n> b'), [{ quote: 'a\n\nb' }])
})

test('">>> " quotes the rest of the message', () => {
  assert.deepEqual(shapes('intro\n>>> one\ntwo\n> three'), [
    'intro',
    { quote: 'one\ntwo\n> three' },
  ])
  assert.deepEqual(shapes('>>> all'), [{ quote: 'all' }])
})

test('">" without a following space or mid-line stays as typed', () => {
  for (const text of ['>no space', 'a > b', '>>no', '>', ' > indented', '>>>no'])
    assert.deepEqual(shapes(text), [text])
})

test('formatting codes inside a quote are kept for rendering', () => {
  assert.deepEqual(shapes('> \x02bold\x02 :smile:'), [{ quote: '\x02bold\x02 :smile:' }])
})

test('a quote of blank lines stays text, and ">>> " alone starts the quote on the next line', () => {
  assert.deepEqual(shapes('a\n> \nb'), ['a\n> \nb'])
  assert.deepEqual(shapes('a\n>>> \nrest'), ['a', { quote: 'rest' }])
})

const blockShapes = (text: string, options?: Parameters<typeof messageBlocks>[1]) =>
  quoteShape(messageBlocks(text, options))
const quoteShape = (blocks: MessageBlock[]): unknown[] =>
  blocks.map((block) =>
    block.kind === 'text'
      ? block.text
      : block.kind === 'code'
        ? { code: block.code }
        : block.kind === 'list'
          ? { list: block.list.items.map((item) => item.text) }
          : { quote: quoteShape(block.blocks) },
  )

test('">>> " quotes code blocks and text after them', () => {
  assert.deepEqual(blockShapes('>>> a\n```\nx\n```\nb'), [{ quote: ['a', { code: 'x' }, 'b'] }])
  // Fences inside "> " lines are a code block in the quote; fences outside end it.
  assert.deepEqual(blockShapes('> ```\n> code\n> ```\n```\ny\n```'), [
    { quote: [{ code: 'code' }] },
    { code: 'y' },
  ])
  assert.deepEqual(blockShapes('> - one\n> - two'), [{ quote: [{ list: ['one', 'two'] }] }])
  // "> " inside code stays code.
  assert.deepEqual(blockShapes('```\n> x\n```'), [{ code: '> x' }])
})

test('an action starts no quote or list on its first line', () => {
  assert.deepEqual(blockShapes('> waves', { continuesLine: true }), ['> waves'])
  assert.deepEqual(blockShapes('- shrugs\nmore', { continuesLine: true }), ['- shrugs\nmore'])
  assert.deepEqual(blockShapes('says\n> hi', { continuesLine: true }), ['says', { quote: ['hi'] }])
})
