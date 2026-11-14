#!/usr/bin/env bun
// Static check, no server: the service worker's APP_ROUTES must match every page in
// app/pages, or a new page silently loses its offline start.
import assert from 'node:assert/strict'
import { readFile, readdir } from 'node:fs/promises'
import { dirname, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontend = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const worker = await readFile(resolve(frontend, 'public/sw.js'), 'utf8')
const source = /^const APP_ROUTES = \/(.+)\/$/m.exec(worker)?.[1]
assert.ok(source, 'public/sw.js no longer defines APP_ROUTES as a regular expression literal')
const appRoutes = new RegExp(source)

const pages = resolve(frontend, 'app/pages')
const files = (await readdir(pages, { recursive: true })).filter((file) => file.endsWith('.vue'))
const paths = []
for (const file of files) {
  const segments = relative(pages, resolve(pages, file))
    .replace(/\.vue$/, '')
    .split('/')
  if (segments.at(-1) === 'index') segments.pop()
  const optional = segments.at(-1)?.startsWith('[[')
  const fixed = optional ? segments.slice(0, -1) : segments
  assert.ok(
    fixed.every((segment) => !segment.startsWith('[')),
    `${file}: add this dynamic route to APP_ROUTES and to this check`,
  )
  const base = '/' + fixed.join('/')
  paths.push(base)
  if (optional) paths.push(`${base === '/' ? '' : base}/example-id`)
}
for (const path of paths) assert.match(path, appRoutes, `APP_ROUTES misses the page at ${path}`)
for (const path of ['/docs', '/openapi.json', '/api/v1/session', '/files/a/b/c.png', '/emotes/x'])
  assert.doesNotMatch(path, appRoutes, `APP_ROUTES must not store ${path} as the app shell`)
console.log(`APP_ROUTES matches all ${files.length} pages and none of the backend pages.`)
