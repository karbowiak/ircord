import { test } from 'node:test'
import assert from 'node:assert/strict'
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
