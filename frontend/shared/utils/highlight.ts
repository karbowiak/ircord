// Syntax highlighting for fenced code blocks. Grammars ship with the app, so rendering code
// never needs another module download. Only a fence's own language tag selects a grammar;
// untagged or unknown code stays plain text (no guessing).
import type { LanguageFn } from 'highlight.js'
import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import c from 'highlight.js/lib/languages/c'
import cpp from 'highlight.js/lib/languages/cpp'
import csharp from 'highlight.js/lib/languages/csharp'
import css from 'highlight.js/lib/languages/css'
import diff from 'highlight.js/lib/languages/diff'
import dockerfile from 'highlight.js/lib/languages/dockerfile'
import go from 'highlight.js/lib/languages/go'
import ini from 'highlight.js/lib/languages/ini'
import java from 'highlight.js/lib/languages/java'
import javascript from 'highlight.js/lib/languages/javascript'
import json from 'highlight.js/lib/languages/json'
import kotlin from 'highlight.js/lib/languages/kotlin'
import lua from 'highlight.js/lib/languages/lua'
import makefile from 'highlight.js/lib/languages/makefile'
import markdown from 'highlight.js/lib/languages/markdown'
import php from 'highlight.js/lib/languages/php'
import python from 'highlight.js/lib/languages/python'
import ruby from 'highlight.js/lib/languages/ruby'
import rust from 'highlight.js/lib/languages/rust'
import scss from 'highlight.js/lib/languages/scss'
import shell from 'highlight.js/lib/languages/shell'
import sql from 'highlight.js/lib/languages/sql'
import swift from 'highlight.js/lib/languages/swift'
import typescript from 'highlight.js/lib/languages/typescript'
import xml from 'highlight.js/lib/languages/xml'
import yaml from 'highlight.js/lib/languages/yaml'

/** The grammars IRCord ships, by highlight.js name. */
const grammars: Record<string, LanguageFn> = {
  bash,
  c,
  cpp,
  csharp,
  css,
  diff,
  dockerfile,
  go,
  ini,
  java,
  javascript,
  json,
  kotlin,
  lua,
  makefile,
  markdown,
  php,
  python,
  ruby,
  rust,
  scss,
  shell,
  sql,
  swift,
  typescript,
  xml,
  yaml,
}
for (const [name, grammar] of Object.entries(grammars)) hljs.registerLanguage(name, grammar)

/** Common fence tags that name a shipped grammar differently. */
const aliases: Record<string, string> = {
  js: 'javascript',
  jsx: 'javascript',
  mjs: 'javascript',
  cjs: 'javascript',
  ts: 'typescript',
  tsx: 'typescript',
  mts: 'typescript',
  py: 'python',
  sh: 'bash',
  zsh: 'bash',
  console: 'shell',
  golang: 'go',
  rs: 'rust',
  rb: 'ruby',
  yml: 'yaml',
  html: 'xml',
  htm: 'xml',
  svg: 'xml',
  vue: 'xml',
  h: 'c',
  'c++': 'cpp',
  cc: 'cpp',
  cxx: 'cpp',
  hpp: 'cpp',
  cs: 'csharp',
  'c#': 'csharp',
  kt: 'kotlin',
  md: 'markdown',
  toml: 'ini',
  docker: 'dockerfile',
  make: 'makefile',
  mk: 'makefile',
  patch: 'diff',
  jsonc: 'json',
  postgres: 'sql',
  postgresql: 'sql',
  mysql: 'sql',
}

/** The shipped grammar a fence tag selects, or undefined to show the code as plain text. */
export function codeLanguage(tag: string | undefined): string | undefined {
  if (!tag) return undefined
  const name = tag.toLowerCase()
  const language = Object.hasOwn(aliases, name) ? aliases[name]! : name
  return Object.hasOwn(grammars, language) ? language : undefined
}

/** Longer code is shown plain: highlighting it would cost more than it helps. */
const maxLength = 20_000
const cacheSize = 200
const cache = new Map<string, string>()

const keyOf = (code: string, language: string) => `${language}\0${code}`

/** Highlighted HTML already computed for this code, so a re-rendered block doesn't flash. */
export function cachedHighlight(code: string, tag: string | undefined): string | undefined {
  const language = codeLanguage(tag)
  return language ? cache.get(keyOf(code, language)) : undefined
}

/**
 * Highlighted HTML for `code`, or null when it stays plain text. highlight.js escapes all of the
 * source and emits only `<span>` elements with class attributes, so the result is safe to render
 * as HTML.
 */
export async function highlightCode(code: string, tag: string | undefined) {
  const language = codeLanguage(tag)
  if (!language || code.length > maxLength) return null
  const key = keyOf(code, language)
  const hit = cache.get(key)
  if (hit !== undefined) return hit
  const html = hljs.highlight(code, { language, ignoreIllegals: true }).value
  if (cache.size >= cacheSize) cache.delete(cache.keys().next().value!)
  cache.set(key, html)
  return html
}
