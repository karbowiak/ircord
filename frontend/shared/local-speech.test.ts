import assert from 'node:assert/strict'
import { createHash } from 'node:crypto'
import { existsSync, readFileSync } from 'node:fs'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import {
  defaultLocalVoice,
  formatDownloadSize,
  localSpeechBase,
  localSpeechCache,
  localSwitchFor,
  speechEngine,
  localSpeechConnectSources,
  localSpeechModel,
  localSpeechPace,
  localSpeechRequest,
  localSpeechShardUrl,
  localSpeechSupport,
  localSpeechVoices,
  localVoice,
  localVoiceOptions,
  mobileDevice,
  prepareLocalSpeech,
  sanitizeLocalVoice,
  sanitizeSpeeds,
  deviceSpeed,
} from './local-speech'

const vendored = fileURLToPath(new URL(`../public${localSpeechBase}`, import.meta.url))
const read = (path: string) => readFileSync(`${vendored}${path}`)

// The lines the plan rendered and transcribed: what IRCord's spoken line becomes for the model.
test('IRC lines are prepared for the voice model', () => {
  const cases: [string, string][] = [
    ['alice in #dev: hello there', 'alice in dev: hello there'],
    [
      'bob in #general: PeterPowers the build took 14 minutes and 3 tests failed',
      'bob in general: PeterPowers the build took 14 minutes and 3 tests failed',
    ],
    [
      'carol in #ops: meeting moved to 15:30 on 2026-11-20, costs went up 25% to $1,200',
      'carol in ops: meeting moved to 15:30 on 2026-11-20, costs went up 25% to $1,200',
    ],
    ['dave: lol ok brb, a link to github.com', 'dave: lol ok brb, a link to github.com'],
    [
      'xX_dark_Xx in #random: PeterPowers: wtf is going on with nginx v1.27.3???',
      'xX dark Xx in random: PeterPowers: wtf is going on with nginx v1.27.3?',
    ],
    [
      'erika in #technik: PeterPowers, das Deployment ist um 14 Uhr fertig geworden',
      'erika in technik: PeterPowers, das Deployment ist um 14 Uhr fertig geworden',
    ],
    ['alice: ok', 'alice: ok'],
    // Beyond the plan's set: hashtags, dashed channel names, emoji, markup and repeats.
    ['zoe in ##dev-ops: ship it!!! 🎉🎉', 'zoe in dev ops: ship it!'],
    ['max: see *this* and `that` ?!?!', 'max: see this and that?'],
    ['ann: hmm.......   ok', 'ann: hmm… ok'],
    ['kim: wait...', 'kim: wait...'],
    ['bob in #dev-ops-2: hi (see #ops)', 'bob in dev ops 2: hi (see ops)'],
    ['COVID-19 on 2026-11-20', 'COVID 19 on 2026-11-20'],
    ['echo in #general: @PeterPowers ok', 'echo in general: PeterPowers ok'],
    ['lee: mail me at lee@example.com', 'lee: mail me at lee@example.com'],
  ]
  for (const [line, spoken] of cases) assert.equal(prepareLocalSpeech(line), spoken, line)
})

test('each request reads one prepared line with the chosen voice', () => {
  const body = localSpeechRequest('alice in #dev: hello there', defaultLocalVoice)
  assert.deepEqual(body, {
    text: 'alice in dev: hello there',
    language: 'en_gb',
    speaker_mode: 'embedding',
    speaker_embedding_id: defaultLocalVoice,
    stream: true,
    format: 'pcm',
    laci: true,
    short_text: 'off',
  })
  const liesel = localSpeechVoices.find((v) => v.name === 'Liesel')!
  assert.equal(localSpeechRequest('x', liesel.id).language, 'de')
})

test('all 27 voices are offered, grouped by language, Rupert by default', () => {
  assert.equal(localSpeechVoices.length, 27)
  const rupert = localVoice(defaultLocalVoice)
  assert.deepEqual(
    [rupert.name, rupert.group, rupert.style, rupert.gender],
    ['Rupert', 'English', 'British', 'male'],
  )
  assert.equal(sanitizeLocalVoice('nobody'), defaultLocalVoice)
  assert.equal(sanitizeLocalVoice(localSpeechVoices[3]!.id), localSpeechVoices[3]!.id)
  const options = localVoiceOptions((key, values) =>
    key.endsWith('Plain')
      ? `${values!.name}, ${values!.language}, ${values!.gender}`
      : key.includes('gender')
        ? key.split('.').pop()!
        : `${values!.name}, ${values!.language} (${values!.style}), ${values!.gender}`,
  )
  assert.equal(
    options.find((o) => o.value === defaultLocalVoice)!.label,
    'Rupert, English (British), male',
  )
  assert.equal(options.find((o) => o.label.startsWith('Liesel'))!.label, 'Liesel, German, female')
  // Options of one language are listed together, so each group heading appears once.
  const groups = options.map((o) => o.group).filter((g, i, all) => g !== all[i - 1])
  assert.equal(new Set(groups).size, groups.length)
})

test('the shipped voice files match the voice list', () => {
  const voices = JSON.parse(read('models/voices/voices.json').toString()) as {
    id: string
    codes: string
    embedding: string
  }[]
  assert.deepEqual(
    voices.map((v) => v.id),
    localSpeechVoices.map((v) => v.id),
  )
  for (const v of voices)
    for (const file of [v.codes, v.embedding])
      assert.ok(existsSync(`${vendored}models/voices/${file}`), file)
  const laci = JSON.parse(read('models/laci/index.json').toString()) as Record<string, string>
  for (const v of localSpeechVoices)
    assert.ok(laci[v.language], `alignment heads for ${v.language}`)
})

test('the vendored files match their checksums and the pinned worker protocol', () => {
  for (const line of read('SHA256SUMS').toString().trim().split('\n')) {
    const [hash, path = ''] = line.split(/\s+/)
    assert.equal(createHash('sha256').update(read(path)).digest('hex'), hash, path)
  }
  const worker = read('worker.js').toString()
  // The client and the cache check depend on these names inside the pinned build.
  assert.ok(worker.includes(localSpeechCache), 'cache name')
  // Messages in (`t.type===\`init\``) and out (`type:\`ready\``), as the minifier writes them.
  const message = (direction: string, name: string) =>
    worker.includes(`${direction}\`${name}\``) ||
    worker.includes(`${direction}"${name}"`) ||
    worker.includes(`${direction}'${name}'`)
  for (const name of ['init', 'req', 'abort']) assert.ok(message('type===', name), name)
  for (const name of ['progress', 'ready', 'fatal', 'res', 'chunk', 'end'])
    assert.ok(message('type:', name), name)
  const shards = (model: string) =>
    (JSON.parse(read(`models/${model}/manifest.json`).toString()) as { shards: string[] }).shards
  for (const model of ['backbone', 'zac'])
    assert.deepEqual(
      shards(model),
      localSpeechModel.shards[model]!.map((s) => s.name),
    )
  const total = Object.values(localSpeechModel.shards)
    .flat()
    .reduce((sum, s) => sum + s.size, 0)
  assert.equal(total, localSpeechModel.downloadBytes)
  assert.equal(formatDownloadSize(localSpeechModel.downloadBytes), '921 MB')
})

test('only manifest shards redirect, to the pinned Hub revision', () => {
  const revision = localSpeechModel.hubRevision
  assert.equal(
    localSpeechShardUrl(`zonos3/${localSpeechModel.version}/models/backbone/shard-000.bin`),
    `https://huggingface.co/Zyphra/ZONOS3-Mini/resolve/${revision}/web/backbone/shard-000.bin`,
  )
  assert.ok(localSpeechShardUrl(`zonos3/${localSpeechModel.version}/models/zac/shard-004.bin`))
  for (const path of [
    `zonos3/${localSpeechModel.version}/models/zac/shard-005.bin`,
    `zonos3/${localSpeechModel.version}/models/encoder/shard-000.bin`,
    `zonos3/${localSpeechModel.version}/models/edit/manifest.json`,
    `zonos3/v0/models/backbone/shard-000.bin`,
    `zonos3/${localSpeechModel.version}/models/backbone/../zac/shard-000.bin`,
    `zonos3/${localSpeechModel.version}/models/backbone/shard-000.bin?x=1`,
    'https://example.com/shard-000.bin',
    '',
  ])
    assert.equal(localSpeechShardUrl(path), '', path)
})

test('the page may reach the Hub only while the operator offers the model', () => {
  assert.equal(localSpeechConnectSources(true), 'https://huggingface.co https://*.hf.co')
  assert.equal(localSpeechConnectSources(false), '')
})

test('phones, tablets and software-only WebGPU cannot use the model', () => {
  const chrome =
    'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36'
  const mac =
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Safari/605.1.15'
  const mouse = { finePointer: true, hover: true }
  const touchOnly = { finePointer: false, hover: false }
  // A computer, and a touch laptop with its trackpad.
  assert.equal(mobileDevice({ userAgent: chrome, maxTouchPoints: 0, ...mouse }), false)
  assert.equal(mobileDevice({ userAgent: chrome, maxTouchPoints: 10, ...mouse }), false)
  assert.equal(mobileDevice({ userAgent: mac, maxTouchPoints: 0, ...mouse }), false)
  // iPadOS as a Mac, even with a trackpad attached.
  assert.equal(mobileDevice({ userAgent: mac, maxTouchPoints: 5, ...mouse }), true)
  assert.equal(mobileDevice({ userAgent: chrome, mobileHint: true, ...mouse }), true)
  assert.equal(
    mobileDevice({
      userAgent: 'Mozilla/5.0 (Linux; Android 15; Pixel 9) Mobile Safari/537.36',
      ...touchOnly,
    }),
    true,
  )
  // An Android tablet in desktop-site mode, or a DeX session without a mouse: a desktop Linux
  // user agent and no mobile hint, but only touch.
  assert.equal(
    mobileDevice({ userAgent: chrome, mobileHint: false, maxTouchPoints: 10, ...touchOnly }),
    true,
  )
  // A coarse pointer that hovers (a stylus) or a precise one that cannot is not a computer either.
  assert.equal(mobileDevice({ userAgent: chrome, finePointer: true, hover: false }), true)
  assert.equal(mobileDevice({ userAgent: chrome, finePointer: false, hover: true }), true)
  const device = { enabled: true, mobile: false, gpu: true, adapter: { isFallbackAdapter: false } }
  assert.equal(localSpeechSupport(device), 'available')
  assert.equal(localSpeechSupport({ ...device, enabled: false }), 'off')
  assert.equal(localSpeechSupport({ ...device, mobile: true }), 'mobile')
  assert.equal(localSpeechSupport({ ...device, gpu: false }), 'no-webgpu')
  assert.equal(localSpeechSupport({ ...device, adapter: null }), 'no-webgpu')
  assert.equal(localSpeechSupport({ ...device, adapter: { isFallbackAdapter: true } }), 'no-webgpu')
})

test('a slow device buffers each line, and a too slow one stops using the model', () => {
  assert.equal(localSpeechPace(null), 'stream')
  assert.equal(localSpeechPace(0.31), 'stream')
  assert.equal(localSpeechPace(0.8), 'buffer')
  assert.equal(localSpeechPace(11.5), 'slow')
  assert.deepEqual(sanitizeSpeeds('fast'), [])
  assert.deepEqual(sanitizeSpeeds([0.4, -1, 'x', 0.5, 0.6, 0.7]), [0.5, 0.6, 0.7])
  assert.equal(deviceSpeed([]), null)
  assert.equal(deviceSpeed([0.4]), 0.4)
  // One line read while the GPU was busy does not mark the device slow.
  assert.equal(localSpeechPace(deviceSpeed([0.35, 2.4])), 'stream')
  assert.equal(localSpeechPace(deviceSpeed([0.35, 0.33, 2.4])), 'stream')
  // Two of three slow lines do.
  assert.equal(localSpeechPace(deviceSpeed([0.35, 2.4, 1.9])), 'slow')
})

test('the voice source selector follows the on-device switch', () => {
  // Existing users: the voice model on lands on WebGPU, everyone else (and an operator that
  // turned the feature off) on API.
  assert.equal(speechEngine(true), 'webgpu')
  assert.equal(speechEngine(false), 'api')
  assert.equal(localSwitchFor('webgpu'), true)
  assert.equal(localSwitchFor('api'), false)

  // Switching away and back round-trips through the switch.
  for (const choice of ['webgpu', 'api', 'webgpu', 'api'] as const)
    assert.equal(speechEngine(localSwitchFor(choice)), choice)
})
