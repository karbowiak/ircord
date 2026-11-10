import { readdir, readFile } from 'node:fs/promises'
import { join, relative } from 'node:path'
import { babelParse, parse } from 'vue/compiler-sfc'
import type { EmbedDeclaration, EmbedProvider } from '../shared/types/embed'

type Node = { type: string; [key: string]: unknown }
function fail(file: string, detail: string): never {
  throw new Error(`[embeds] ${file}: ${detail}`)
}
/** Deliberately accepts literals only: no calls, references, spreads, getters, or evaluation. */
function literal(node: Node, file: string): unknown {
  if (['TSAsExpression', 'TSSatisfiesExpression', 'ParenthesizedExpression'].includes(node.type))
    return literal(node.expression as Node, file)
  if (['StringLiteral', 'NumericLiteral', 'BooleanLiteral'].includes(node.type)) return node.value
  if (node.type === 'ArrayExpression')
    return (node.elements as Node[]).map((item) => {
      if (!item) fail(file, 'embed declarations cannot contain sparse arrays')
      return literal(item, file)
    })
  if (node.type === 'ObjectExpression') {
    const result: Record<string, unknown> = Object.create(null)
    for (const property of node.properties as Node[]) {
      if (property.type !== 'ObjectProperty' || property.computed || property.method)
        fail(file, 'embed declarations must contain plain literal properties')
      const key = property.key as Node
      const name = key.type === 'Identifier' ? String(key.name) : String(key.value)
      if (Object.hasOwn(result, name)) fail(file, `duplicate declaration property ${name}`)
      result[name] = literal(property.value as Node, file)
    }
    return result
  }
  return fail(file, `embed declarations must be static literals (found ${node.type})`)
}
function object(value: unknown, file: string, name: string): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    fail(file, `${name} must be an object`)
  return value as Record<string, unknown>
}
function keys(value: Record<string, unknown>, allowed: string[], file: string, name: string) {
  for (const key of Object.keys(value))
    if (!allowed.includes(key)) fail(file, `unknown ${name} property ${key}`)
}
function strings(value: unknown, file: string, name: string, pattern?: RegExp): string[] {
  if (!Array.isArray(value) || !value.length || value.length > 64)
    fail(file, `${name} must contain between 1 and 64 strings`)
  for (const item of value)
    if (typeof item !== 'string' || !item || item.length > 256 || (pattern && !pattern.test(item)))
      fail(file, `invalid ${name} entry ${JSON.stringify(item)}`)
  if (new Set(value).size !== value.length) fail(file, `${name} contains duplicates`)
  return value as string[]
}
function layout(value: unknown, file: string) {
  const item = object(value, file, 'layout')
  keys(item, ['ratio', 'height'], file, 'layout')
  if (Object.keys(item).length !== 1) fail(file, 'layout needs exactly one of ratio or height')
  if (item.ratio !== undefined) {
    if (
      !Array.isArray(item.ratio) ||
      item.ratio.length !== 2 ||
      item.ratio.some((n) => typeof n !== 'number' || !Number.isFinite(n) || n < 1 || n > 100)
    )
      fail(file, 'layout.ratio must be two finite numbers between 1 and 100')
  } else if (
    !Number.isInteger(item.height) ||
    Number(item.height) < 64 ||
    Number(item.height) > 800
  )
    fail(file, 'layout.height must be an integer between 64 and 800')
}
function matches(value: unknown, file: string) {
  if (!Array.isArray(value) || !value.length || value.length > 64)
    fail(file, 'matches must contain between 1 and 64 rules')
  for (const entry of value) {
    const rule = object(entry, file, 'match')
    keys(rule, ['hosts', 'paths', 'query', 'extensions'], file, 'match')
    if (!rule.hosts && !rule.extensions)
      fail(file, 'each match requires hosts or extensions for indexing')
    if (rule.hosts)
      strings(rule.hosts, file, 'hosts', /^(?:\*\.)?[a-z0-9]+(?:[-.][a-z0-9]+)*\.[a-z]{2,}$/)
    if (rule.extensions) strings(rule.extensions, file, 'extensions', /^[a-z0-9]{1,16}$/)
    if (rule.query) strings(rule.query, file, 'query', /^[a-zA-Z0-9_-]+$/)
    if (rule.paths) {
      for (const path of strings(rule.paths, file, 'paths')) {
        if (
          !path.startsWith('/') ||
          path.includes('?') ||
          path.includes('#') ||
          path
            .split('/')
            .some(
              (part, i, parts) =>
                (part.includes('*') && part !== '*' && part !== '**') ||
                (part === '**' && i !== parts.length - 1),
            )
        )
          fail(file, `invalid path ${path}; use whole segments * or a final **`)
      }
    }
  }
}
export function validateEmbed(value: unknown, file: string): EmbedDeclaration {
  const item = object(value, file, 'embed')
  keys(
    item,
    [
      'id',
      'priority',
      'matches',
      'attachments',
      'attachmentTypes',
      'fallback',
      'layout',
      'imageSizing',
      'resolver',
      'accepts',
      'providers',
      'frames',
      'hideUrl',
      'variants',
    ],
    file,
    'embed',
  )
  if (typeof item.id !== 'string' || !/^[a-z][a-z0-9-]{0,63}$/.test(item.id))
    fail(file, 'id must be a lowercase slug of at most 64 characters')
  if (
    !Number.isInteger(item.priority) ||
    Number(item.priority) < 0 ||
    Number(item.priority) > 10000
  )
    fail(file, 'priority must be an integer between 0 and 10000')
  for (const name of ['fallback', 'hideUrl', 'imageSizing'])
    if (item[name] !== undefined && typeof item[name] !== 'boolean')
      fail(file, `${name} must be boolean`)
  if (item.fallback) {
    if (item.matches || item.attachments || item.attachmentTypes || item.hideUrl || item.variants)
      fail(file, 'the fallback cannot have matches, attachments, variants, or hideUrl')
  } else if (!item.matches && !item.attachments && !item.attachmentTypes)
    fail(file, 'a provider requires matches or attachments')
  if (item.matches) matches(item.matches, file)
  if (item.attachments)
    strings(item.attachments, file, 'attachments', /^(image|video|audio|file|gallery)$/)
  if (item.attachmentTypes)
    strings(item.attachmentTypes, file, 'attachmentTypes', /^[a-z0-9.+-]+\/[a-z0-9.+-]+$/)
  layout(item.layout, file)
  if (item.imageSizing && !('height' in (item.layout as Record<string, unknown>)))
    fail(file, 'image sizing requires a fixed height cap')
  if (item.resolver !== 'message' && item.resolver !== 'none')
    fail(file, 'resolver must be message or none')
  const accepts = strings(item.accepts, file, 'accepts', /^(media|file|card|widget)$/)
  if (item.providers) strings(item.providers, file, 'providers', /^[a-z][a-z0-9-]{0,63}$/)
  if (accepts.includes('widget') && !item.providers)
    fail(file, 'widget adapters must declare providers')
  if (accepts.includes('widget') && !item.frames)
    fail(file, 'widget adapters must declare frame matches')
  if (item.frames) matches(item.frames, file)
  if (item.fallback && accepts.some((kind) => kind !== 'card' && kind !== 'file'))
    fail(file, 'fallback accepts only fixed card/file presentations')
  if (item.variants) {
    if (!Array.isArray(item.variants) || item.variants.length > 32)
      fail(file, 'variants must be an array of at most 32 items')
    for (const entry of item.variants) {
      const variant = object(entry, file, 'variant')
      keys(variant, ['matches', 'layout'], file, 'variant')
      matches(variant.matches, file)
      layout(variant.layout, file)
    }
  }
  return item as unknown as EmbedDeclaration
}
export function extractEmbed(source: string, file: string): EmbedDeclaration {
  const { descriptor, errors } = parse(source, { filename: file })
  if (errors.length) fail(file, String(errors[0]))
  if (!descriptor.script || descriptor.script.src)
    fail(file, 'declare export const embedProvider = { ... } in a normal <script lang="ts"> block')
  const ast = babelParse(descriptor.script.content, {
    sourceType: 'module',
    plugins: ['typescript'],
  })
  const declarations: unknown[] = []
  for (const statement of ast.program.body) {
    if (
      statement.type !== 'ExportNamedDeclaration' ||
      statement.declaration?.type !== 'VariableDeclaration'
    )
      continue
    if (
      statement.declaration.kind !== 'const' &&
      statement.declaration.declarations.some(
        (variable) => variable.id.type === 'Identifier' && variable.id.name === 'embedProvider',
      )
    )
      fail(file, 'embedProvider must be declared with const')
    for (const variable of statement.declaration.declarations)
      if (
        variable.id.type === 'Identifier' &&
        variable.id.name === 'embedProvider' &&
        variable.init
      )
        declarations.push(literal(variable.init as unknown as Node, file))
  }
  if (declarations.length !== 1)
    fail(file, 'expected exactly one export const embedProvider = { ... } declaration')
  return validateEmbed(declarations[0], file)
}
export async function discoverEmbeds(directory: string): Promise<EmbedProvider[]> {
  const files = (await readdir(directory, { recursive: true, withFileTypes: true }))
    .filter((entry) => entry.isFile() && entry.name.endsWith('.vue'))
    .map((entry) => join(entry.parentPath, entry.name))
    .sort()
  const providers = await Promise.all(
    files.map(async (file) => ({
      ...extractEmbed(await readFile(file, 'utf8'), file),
      component: relative(directory, file).replaceAll('\\', '/'),
    })),
  )
  const ids = new Set<string>()
  const signatures = new Map<string, string>()
  for (const provider of providers) {
    if (ids.has(provider.id)) fail(provider.component, `duplicate provider id ${provider.id}`)
    ids.add(provider.id)
    // Equal-priority identical rules are usually an accidental copy of another adapter.
    for (const rule of provider.matches ?? []) {
      const signature = JSON.stringify([
        provider.priority,
        [...(rule.hosts ?? [])].sort(),
        [...(rule.paths ?? [])].sort(),
        [...(rule.query ?? [])].sort(),
        [...(rule.extensions ?? [])].sort(),
      ])
      const previous = signatures.get(signature)
      if (previous)
        fail(
          provider.component,
          `duplicate match at priority ${provider.priority}; already declared by ${previous}`,
        )
      signatures.set(signature, provider.component)
    }
  }
  if (providers.filter((provider) => provider.fallback).length !== 1)
    fail(directory, 'declare exactly one fallback provider')
  return providers.sort(
    (a, b) => b.priority - a.priority || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0),
  )
}
export function embedManifest(providers: EmbedProvider[], directory: string, types: string) {
  return [
    `import type { EmbedProvider } from ${JSON.stringify(types)}`,
    `export const embedProviders: EmbedProvider[] = ${JSON.stringify(providers, null, 2)}`,
    'export const embedLoaders = {',
    ...providers.map(
      (provider) =>
        `  ${JSON.stringify(provider.component)}: () => import(${JSON.stringify(join(directory, provider.component))}),`,
    ),
    '}',
  ].join('\n')
}
