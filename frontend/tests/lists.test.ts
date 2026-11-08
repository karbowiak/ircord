import { test } from 'node:test'
import assert from 'node:assert/strict'
import { markdownToIrc } from '../shared/utils/irc'
import { maxListDepth, normalizeBullets, splitLists, type MessageList } from '../shared/utils/lists'

/** A list as nested arrays: item text, or [text, ...child lists]. */
function shape(list: MessageList): unknown {
  return {
    [list.ordered ? `ol${list.start}` : 'ul']: list.items.map((item) =>
      item.children.length ? [item.text, ...item.children.map(shape)] : item.text,
    ),
  }
}
const shapes = (text: string) =>
  splitLists(text).map((block) => (block.kind === 'text' ? block.text : shape(block.list)))

test('Markdown list lines become lists, and the text around them stays text', () => {
  assert.deepEqual(shapes('groceries:\n- milk\n* eggs\n+ bread\nthanks'), [
    'groceries:',
    { ul: ['milk', 'eggs', 'bread'] },
    'thanks',
  ])
  assert.deepEqual(shapes('3. three\n4) four'), [{ ol3: ['three', 'four'] }])
  // A blank line ends the list and stays in the text.
  assert.deepEqual(shapes('- a\n\nafter'), [{ ul: ['a'] }, '\nafter'])
  // Switching between bullets and numbers starts a new list.
  assert.deepEqual(shapes('- a\n1. b'), [{ ul: ['a'] }, { ol1: ['b'] }])
  // An unindented line ends the list; the next item starts another.
  assert.deepEqual(shapes('- a\ntext\n- b'), [{ ul: ['a'] }, 'text', { ul: ['b'] }])
  // A marker with nothing after it is text, even right after an item.
  assert.deepEqual(shapes('- a\n- '), [{ ul: ['a'] }, '- '])
})

test('an indented line continues the item above it', () => {
  assert.deepEqual(shapes('- a\n  more\n- b'), [{ ul: ['a\nmore', 'b'] }])
  assert.deepEqual(shapes('1. a\n   - b\n     more b\n2. c'), [
    { ol1: [['a', { ul: ['b\nmore b'] }], 'c'] },
  ])
  // Indented no further than the list itself, it is ordinary text.
  assert.deepEqual(shapes('  - a\n  text'), [{ ul: ['a'] }, '  text'])
})

test('indentation nests lists under the item above', () => {
  assert.deepEqual(shapes('- a\n  - b\n    1. c\n  - d\n- e'), [
    { ul: [['a', { ul: [['b', { ol1: ['c'] }], 'd'] }], 'e'] },
  ])
  // Tabs count as indentation too, as four columns.
  assert.deepEqual(shapes('1. a\n\t- b'), [{ ol1: [['a', { ul: ['b'] }]] }])
  assert.deepEqual(shapes('- a\n  - b\n\t- c'), [{ ul: [['a', { ul: [['b', { ul: ['c'] }]] }]] }])
  // Hostile indentation nests only so deep.
  const deep = Array.from({ length: 40 }, (_, i) => `${' '.repeat(i * 2)}- ${i}`).join('\n')
  const depthOf = (list: MessageList): number =>
    1 + Math.max(0, ...list.items.flatMap((item) => item.children.map(depthOf)))
  const depth = depthOf((splitLists(deep)[0] as { list: MessageList }).list)
  assert.equal(depth, maxListDepth)
})

test('lines that only look like markers stay text', () => {
  for (const text of [
    '-5 degrees',
    '- ',
    '*italic*',
    '2024.was',
    'a - b',
    '1.5 litres',
    '--',
    '1234567890. too long a number',
  ])
    assert.deepEqual(shapes(text), [text])
})

test('list markers survive the composer conversion, and items keep their formatting', () => {
  assert.equal(markdownToIrc('* one *two*\n* three'), '* one \x1dtwo\x1d\n* three')
  assert.equal(markdownToIrc('- **bold** item'), '- \x02bold\x02 item')
  assert.deepEqual(shapes(markdownToIrc('* **x**')), [{ ul: ['\x02x\x02'] }])
})

test('bullet glyphs from pasted plain text become Markdown markers', () => {
  assert.equal(normalizeBullets('• a\n  ◦ b\n▪ c\nnot • here'), '- a\n  - b\n- c\nnot • here')
  assert.equal(normalizeBullets('no bullets'), 'no bullets')
})

test('list splitting stays linear on large input', () => {
  const started = performance.now()
  splitLists('- x\n'.repeat(50_000))
  splitLists(' '.repeat(100_000) + '- x')
  splitLists('1. '.repeat(50_000))
  assert.ok(performance.now() - started < 1000)
})
