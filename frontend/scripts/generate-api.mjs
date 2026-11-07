import { execFileSync } from 'node:child_process'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import openapiTS, { astToString } from 'openapi-typescript'
import ts from 'typescript'
import Ajv from 'ajv/dist/2020.js'
import standaloneCode from 'ajv/dist/standalone/index.js'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const check = process.argv.includes('--check')
const json = execFileSync('go', ['run', './cmd/openapi'], { cwd: resolve(root, 'backend'), encoding: 'utf8' })
const schema = JSON.parse(json)
const ast = await openapiTS(schema, {
  transform(value) { if (value.format === 'binary') return ts.factory.createTypeReferenceNode('Blob') },
})
const types = astToString(ast)
// Only event-reachable components enter the runtime validator. It is compiled
// here, never with eval in the browser (the app has a strict CSP).
const defs = {}
function collect(name) {
  if (name in defs) return
  defs[name] = schema.components.schemas[name]
  if (!defs[name]) throw new Error(`Missing schema ${name}`)
  const walk = value => {
    if (value && typeof value === 'object') {
      if (typeof value.$ref === 'string') collect(value.$ref.replace('#/components/schemas/', ''))
      Object.values(value).forEach(walk)
    }
  }
  walk(defs[name])
}
collect('ServerEvent')
const eventSchema = JSON.parse(JSON.stringify({ $schema: 'https://json-schema.org/draft/2020-12/schema', $ref: '#/$defs/ServerEvent', $defs: defs }).replaceAll('#/components/schemas/', '#/$defs/'))
// Ajv dispatches using the literal enums; OpenAPI's explicit mapping is for SDKs.
delete eventSchema.$defs.ServerEvent.discriminator.mapping
const ajv = new Ajv({ strict: false, discriminator: true, validateFormats: false, code: { esm: true, source: true } })
const validate = ajv.compile(eventSchema)
const validator = '// Generated from Go OpenAPI. Run npm run api:generate.\n' + standaloneCode(ajv, validate)
const declarations = `import type { components } from './api'\ndeclare const validate: (value: unknown) => value is components['schemas']['ServerEvent']\nexport default validate\n`
const artifacts = new Map([
  ['backend/openapi.json', json],
  ['frontend/shared/generated/api.d.ts', types],
  ['frontend/shared/generated/event-validator.js', validator],
  ['frontend/shared/generated/event-validator.d.ts', declarations],
])
let stale = false
for (const [name, content] of artifacts) {
  const path = resolve(root, name)
  if (check) {
    if (await readFile(path, 'utf8').catch(() => '') !== content) { console.error(`Stale generated contract: ${name}`); stale = true }
  } else { await mkdir(dirname(path), { recursive: true }); await writeFile(path, content) }
}
if (stale) { console.error('Run npm run api:generate in frontend/.'); process.exitCode = 1 }
else console.log(check ? 'Generated API contract is current.' : 'Generated OpenAPI, TypeScript and WebSocket validator.')
