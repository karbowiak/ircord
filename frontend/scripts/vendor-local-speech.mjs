// Vendors the in-browser speech engine for read-aloud's "Voice model on this device" option.
//
// IRCord ships Zyphra's ZONOS3-Mini web engine (two worker files built from the GitHub repository
// at a pinned commit) and the model's small control files (manifests, voice codes and embeddings,
// the steering bundle and alignment heads, about 0.6 MB, from the Hugging Face repository at a
// pinned revision). The large weight shards are not vendored: the browser downloads them from
// Hugging Face at the same revision, through server/routes/speech/[...path].ts.
//
// Usage, from frontend/:
//   bun scripts/vendor-local-speech.mjs                 clone, build and download, then write
//   bun scripts/vendor-local-speech.mjs --engine <dir>  use an existing build (servers/web/dist)
//   bun scripts/vendor-local-speech.mjs --hub <dir>     use a local snapshot of the Hub's web/
//   bun scripts/vendor-local-speech.mjs --check         verify the vendored files and module
//
// To move to a newer upstream: change the pins below, run without flags, review the worker
// protocol (src/worker.ts InMsg/OutMsg) against app/utils/local-speech-engine.ts, then run
// `bun run build && bun run test:local-speech` and the real-GPU check in docs/frontend.md.
import { createHash } from 'node:crypto'
import { execFileSync } from 'node:child_process'
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const engineRepo = 'https://github.com/Zyphra/ZONOS3-Mini'
const engineCommit = '96d9e0ec1e39083ed336ac78f24e3dbc93721c0f'
const hubRepo = 'Zyphra/ZONOS3-Mini'
const hubRevision = 'd2d142a86b1e6aeb4252679673c86ecd86177bf8'
/** The public path segment; bump it with every pin change so browsers never mix versions. */
const version = 'v1'
/** The models read-aloud loads at start: the voice model and the default audio decoder. */
const models = ['backbone', 'zac']

const frontend = join(dirname(fileURLToPath(import.meta.url)), '..')
const out = join(frontend, 'public', 'speech', 'zonos3', version)
const moduleFile = join(frontend, 'shared', 'generated', 'local-speech-model.ts')

const args = process.argv.slice(2)
const option = (name) => {
  const i = args.indexOf(name)
  return i >= 0 ? args[i + 1] : undefined
}
const sha256 = (data) => createHash('sha256').update(data).digest('hex')

/** Voice file stems such as `Rupert_en-British_M_1`: name, language, optional style, gender. */
function parseVoice(stem) {
  const match = /^([A-Za-z]+)_([a-z]{2})(?:-([A-Za-z]+))?_([MF])_\d+$/.exec(stem)
  if (!match) throw new Error(`unexpected voice file name ${stem}`)
  const [, name, lang, style, gender] = match
  return { name, lang, style: style ?? '', gender: gender === 'M' ? 'male' : 'female' }
}
/** Each voice's text language: the engine's normalization and alignment-head code. */
function speechLanguage(lang, style) {
  if (lang === 'en') return style === 'British' || style === 'Irish' ? 'en_gb' : 'en_us'
  return { fr: 'fr_fr', pt: 'pt_br', zh: 'cmn' }[lang] ?? lang
}
/** Upstream's style tags, as people say them. */
const styleNames = { announcervoice: 'announcer', asrm: 'ASMR' }

function run(command, commandArgs, cwd) {
  execFileSync(command, commandArgs, { cwd, stdio: 'inherit' })
}

function buildEngine() {
  const given = option('--engine')
  if (given) return { dist: given, license: join(given, '..', '..', '..', 'LICENSE') }
  const work = mkdtempSync(join(process.env.TMPDIR || tmpdir(), 'zonos3-'))
  const repo = join(work, 'repo')
  run('git', ['clone', '--quiet', engineRepo, repo])
  run('git', ['checkout', '--quiet', engineCommit], repo)
  run('pnpm', ['install', '--frozen-lockfile'], repo)
  run('pnpm', ['--dir', 'servers/web', 'build'], repo)
  return { dist: join(repo, 'servers', 'web', 'dist'), license: join(repo, 'LICENSE'), work }
}

/** Reads one file of the Hub's web/ folder, from a local snapshot or the pinned revision. */
function hubReader() {
  const local = option('--hub')
  if (local)
    return {
      read: async (path) => readFileSync(join(local, path)),
      size: async (path) => statSync(join(local, path)).size,
    }
  const url = (path) =>
    `https://huggingface.co/${hubRepo}/resolve/${hubRevision}/web/${path.split('/').map(encodeURIComponent).join('/')}`
  return {
    async read(path) {
      const response = await fetch(url(path))
      if (!response.ok) throw new Error(`${path}: HTTP ${response.status}`)
      return Buffer.from(await response.arrayBuffer())
    },
    async size(path) {
      const response = await fetch(url(path), { method: 'HEAD', redirect: 'manual' })
      const size = Number(
        response.headers.get('x-linked-size') ?? response.headers.get('content-length'),
      )
      if (!size) throw new Error(`${path}: no size`)
      return size
    },
  }
}

async function vendor() {
  const engine = buildEngine()
  const hub = hubReader()
  rmSync(out, { recursive: true, force: true })
  const files = new Map()
  const put = (path, data) => files.set(path, data)

  put('worker.js', readFileSync(join(engine.dist, 'worker.js')))
  put('codec-worker.js', readFileSync(join(engine.dist, 'codec-worker.js')))
  put('LICENSE.txt', readFileSync(engine.license))

  // Manifests, and the shards they name (sizes only: the browser downloads them).
  const shards = {}
  let downloadBytes = 0
  for (const model of models) {
    const manifest = await hub.read(`${model}/manifest.json`)
    put(`models/${model}/manifest.json`, manifest)
    shards[model] = []
    for (const name of JSON.parse(manifest).shards) {
      if (!/^shard-\d{3}\.bin$/.test(name)) throw new Error(`unexpected shard name ${name}`)
      const size = await hub.size(`${model}/${name}`)
      shards[model].push({ name, size })
      downloadBytes += size
    }
  }

  // Default voices: codes and embedding under flat names. Their clips (21 MB, previews only)
  // are left out; the engine only needs them to decode a clip, which read-aloud never does.
  const upstreamVoices = JSON.parse(await hub.read('voices/voices.json'))
  const voices = []
  const languages = new Set()
  for (const v of upstreamVoices) {
    const stem = v.codes
      .split('/')
      .pop()
      .replace(/\.codes\.npy$/, '')
    const slug = stem.toLowerCase().replace(/_/g, '-')
    const { name, lang, style, gender } = parseVoice(stem)
    const language = speechLanguage(lang, style)
    languages.add(language)
    put(`models/voices/${slug}.codes.npy`, await hub.read(`voices/${v.codes}`))
    put(`models/voices/${slug}.embedding.npy`, await hub.read(`voices/${v.embedding}`))
    voices.push({
      json: {
        id: v.id,
        label: v.label,
        file: `${slug}${v.file.slice(v.file.lastIndexOf('.'))}`,
        codes: `${slug}.codes.npy`,
        embedding: `${slug}.embedding.npy`,
      },
      entry: {
        id: v.id,
        name,
        group: v.file.split('/')[0],
        style: styleNames[style] ?? style,
        gender,
        language,
      },
    })
  }
  put(
    'models/voices/voices.json',
    JSON.stringify(
      voices.map((v) => v.json),
      null,
      1,
    ) + '\n',
  )

  // The steering bundle, and alignment heads for the voices' languages only.
  put('models/companions/steer.gguf', await hub.read('companions/steer.gguf'))
  const laciIndex = JSON.parse(await hub.read('laci/index.json'))
  const laci = {}
  for (const language of [...languages].sort()) {
    const file = laciIndex[language]
    if (!file) throw new Error(`no alignment heads for ${language}`)
    laci[language] = file
    if (!files.has(`models/laci/${file}`))
      put(`models/laci/${file}`, await hub.read(`laci/${file}`))
  }
  put('models/laci/index.json', JSON.stringify(laci, null, 1) + '\n')

  put(
    'NOTICE.txt',
    [
      'ZONOS3-Mini in-browser engine and model files, used by IRCord read-aloud.',
      'Copyright (c) 2026 Zyphra, Inc.',
      '',
      `worker.js and codec-worker.js: built from ${engineRepo} at commit ${engineCommit}`,
      '(servers/web, "pnpm --dir servers/web build"). MIT License, see LICENSE.txt.',
      '',
      `models/: files from https://huggingface.co/${hubRepo} at revision ${hubRevision}`,
      '(web/ folder; voice files renamed, unused alignment heads left out). Apache License 2.0.',
      'The weight shards are not included; browsers download them from that revision.',
      '',
      'Regenerate with frontend/scripts/vendor-local-speech.mjs.',
      '',
    ].join('\n'),
  )

  for (const [path, data] of files) {
    mkdirSync(dirname(join(out, path)), { recursive: true })
    writeFileSync(join(out, path), data)
  }
  writeFileSync(join(out, 'SHA256SUMS'), checksums(files))
  writeFileSync(
    moduleFile,
    modelModule({ shards, downloadBytes, voices: voices.map((v) => v.entry) }),
  )
  if (engine.work) rmSync(engine.work, { recursive: true, force: true })
  console.log(
    `vendored ${files.size} files to ${relative(frontend, out)}; ${downloadBytes} bytes of shards on the Hub`,
  )
}

function checksums(files) {
  return (
    [...files.keys()]
      .sort()
      .map((path) => `${sha256(files.get(path))}  ${path}`)
      .join('\n') + '\n'
  )
}

function modelModule({ shards, downloadBytes, voices }) {
  // One entry per line, keys unquoted.
  const item = (value) =>
    JSON.stringify(value)
      .replace(/"([a-zA-Z]+)":/g, '$1: ')
      .replace(/,(?=[a-z]+: )/g, ', ')
  const list = (values, indent) =>
    `[\n${values.map((v) => `${indent}  ${item(v)},`).join('\n')}\n${indent}]`
  const json = (value) =>
    Array.isArray(value)
      ? list(value, '')
      : `{\n${Object.entries(value)
          .map(([k, v]) => `    ${k}: ${list(v, '    ')},`)
          .join('\n')}\n  }`
  return `// Generated by scripts/vendor-local-speech.mjs. Do not edit by hand.
/** The vendored ZONOS3-Mini engine and model revision read-aloud uses on this device. */
export const localSpeechModel = {
  version: '${version}',
  engineCommit: '${engineCommit}',
  hubRepo: '${hubRepo}',
  hubRevision: '${hubRevision}',
  /** Bytes the browser downloads from the Hub on first use. */
  downloadBytes: ${downloadBytes},
  /** The weight shards of each model, which the server redirects to the Hub. */
  shards: ${json(shards)} as Record<string, { name: string; size: number }[]>,
}

/** The default voices: upstream's id, the name, the language group, a style and the text language. */
export const localSpeechVoices = ${json(voices)} as const
`
}

/** Whether the module names this value as a string literal (Prettier may change its quotes). */
const quoted = (module, value) => module.includes(`'${value}'`) || module.includes(`"${value}"`)

function check() {
  const sums = readFileSync(join(out, 'SHA256SUMS'), 'utf8').trim().split('\n')
  for (const line of sums) {
    const [hash, path] = line.split(/\s+/)
    if (sha256(readFileSync(join(out, path))) !== hash)
      throw new Error(`${path}: checksum mismatch`)
  }
  const module = readFileSync(moduleFile, 'utf8')
  for (const model of models) {
    const manifest = JSON.parse(readFileSync(join(out, 'models', model, 'manifest.json'), 'utf8'))
    for (const name of manifest.shards)
      if (!quoted(module, name)) throw new Error(`${model}/${name} missing from the module`)
  }
  const voices = JSON.parse(readFileSync(join(out, 'models', 'voices', 'voices.json'), 'utf8'))
  for (const v of voices) {
    if (!quoted(module, v.id)) throw new Error(`voice ${v.id} missing from the module`)
    for (const file of [v.codes, v.embedding])
      if (!existsSync(join(out, 'models', 'voices', file)))
        throw new Error(`voices/${file} missing`)
  }
  console.log(`${sums.length} vendored files match SHA256SUMS; ${voices.length} voices`)
}

if (args.includes('--check')) check()
else await vendor()
