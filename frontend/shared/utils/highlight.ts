// Syntax highlighting for fenced code blocks. highlight.js and each grammar load on demand, so
// chats without code never download them. Only a fence's own language tag selects a grammar;
// untagged or unknown code stays plain text (no guessing).
import type { HLJSApi, LanguageFn } from 'highlight.js'

type Loader = () => Promise<{ default: LanguageFn }>

/** The grammars IRCord ships, by highlight.js name. */
const grammars: Record<string, Loader> = {
  bash: () => import('highlight.js/lib/languages/bash'),
  c: () => import('highlight.js/lib/languages/c'),
  cpp: () => import('highlight.js/lib/languages/cpp'),
  csharp: () => import('highlight.js/lib/languages/csharp'),
  css: () => import('highlight.js/lib/languages/css'),
  diff: () => import('highlight.js/lib/languages/diff'),
  dockerfile: () => import('highlight.js/lib/languages/dockerfile'),
  go: () => import('highlight.js/lib/languages/go'),
  ini: () => import('highlight.js/lib/languages/ini'),
  java: () => import('highlight.js/lib/languages/java'),
  javascript: () => import('highlight.js/lib/languages/javascript'),
  json: () => import('highlight.js/lib/languages/json'),
  kotlin: () => import('highlight.js/lib/languages/kotlin'),
  lua: () => import('highlight.js/lib/languages/lua'),
  makefile: () => import('highlight.js/lib/languages/makefile'),
  markdown: () => import('highlight.js/lib/languages/markdown'),
  php: () => import('highlight.js/lib/languages/php'),
  python: () => import('highlight.js/lib/languages/python'),
  ruby: () => import('highlight.js/lib/languages/ruby'),
  rust: () => import('highlight.js/lib/languages/rust'),
  scss: () => import('highlight.js/lib/languages/scss'),
  shell: () => import('highlight.js/lib/languages/shell'),
  sql: () => import('highlight.js/lib/languages/sql'),
  swift: () => import('highlight.js/lib/languages/swift'),
  typescript: () => import('highlight.js/lib/languages/typescript'),
  xml: () => import('highlight.js/lib/languages/xml'),
  yaml: () => import('highlight.js/lib/languages/yaml'),
}

/** Languages a grammar embeds (HTML's <script> and <style>, a shell session's commands). */
const embeds: Record<string, string[]> = { xml: ['javascript', 'css'], shell: ['bash'] }

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
let core: Promise<HLJSApi> | undefined
const loading = new Map<string, Promise<void>>()

function loadCore() {
  core ??= import('highlight.js/lib/core')
    .then((module) => module.default)
    .catch((error) => {
      // A failed download can be retried by the next block that needs it.
      core = undefined
      throw error
    })
  return core
}

/** Registers `language` and the languages it embeds, so the first highlight is already complete. */
function grammar(language: string): Promise<void> {
  let pending = loading.get(language)
  if (!pending) {
    pending = Promise.all([
      loadCore(),
      grammars[language]!(),
      ...(embeds[language] ?? []).map(grammar),
    ]).then(([hljs, module]) => hljs.registerLanguage(language, module.default))
    // A failed download can be retried by the next block that needs it.
    pending.catch(() => loading.delete(language))
    loading.set(language, pending)
  }
  return pending
}

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
  await grammar(language)
  const hljs = await loadCore()
  const html = hljs.highlight(code, { language, ignoreIllegals: true }).value
  if (cache.size >= cacheSize) cache.delete(cache.keys().next().value!)
  cache.set(key, html)
  return html
}
