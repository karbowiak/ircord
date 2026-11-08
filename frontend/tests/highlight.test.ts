import { test } from 'node:test'
import assert from 'node:assert/strict'
import { cachedHighlight, codeLanguage, highlightCode } from '../shared/utils/highlight'

const textOf = (html: string) =>
  html
    .replace(/<[^>]*>/g, '')
    .replaceAll('&lt;', '<')
    .replaceAll('&gt;', '>')
    .replaceAll('&quot;', '"')
    .replaceAll('&#x27;', "'")
    .replaceAll('&amp;', '&')

test('fence tags select a shipped grammar; untagged and unknown code stays plain', () => {
  assert.equal(codeLanguage('js'), 'javascript')
  assert.equal(codeLanguage('TS'), 'typescript')
  assert.equal(codeLanguage('py'), 'python')
  assert.equal(codeLanguage('go'), 'go')
  assert.equal(codeLanguage('c++'), 'cpp')
  assert.equal(codeLanguage('html'), 'xml')
  assert.equal(codeLanguage(undefined), undefined)
  assert.equal(codeLanguage(''), undefined)
  assert.equal(codeLanguage('brainfuck'), undefined)
  assert.equal(codeLanguage('constructor'), undefined)
  assert.equal(codeLanguage('__proto__'), undefined)
})

test('tagged code is highlighted without changing its text', async () => {
  const code = 'const greet = (name) => `hi ${name}` // say hi\nreturn 42'
  const html = await highlightCode(code, 'js')
  assert.ok(html)
  assert.match(html, /<span class="hljs-keyword">const<\/span>/)
  assert.match(html, /<span class="hljs-comment">\/\/ say hi<\/span>/)
  assert.match(html, /<span class="hljs-number">42<\/span>/)
  assert.equal(textOf(html), code)
  assert.equal(cachedHighlight(code, 'javascript'), html)
})

test('highlighted output escapes markup in the source', async () => {
  const code = 'x = "<script>alert(1)</script>" # <img src=x onerror=alert(1)>'
  const html = await highlightCode(code, 'python')
  assert.ok(html)
  assert.doesNotMatch(html, /<(?!\/?span\b)/)
  assert.equal(textOf(html), code)
  // Only highlight.js's own span markup survives, even for a markup language.
  const markup = await highlightCode('<a href="javascript:alert(1)" onclick="x()">hi</a>', 'html')
  assert.ok(markup)
  assert.doesNotMatch(markup, /<(?!\/?span\b)/)
})

test('untagged, unknown and oversized code is not highlighted', async () => {
  assert.equal(await highlightCode('const x = 1', undefined), null)
  assert.equal(await highlightCode('const x = 1', 'nope'), null)
  assert.equal(await highlightCode('x'.repeat(20_001), 'js'), null)
})
