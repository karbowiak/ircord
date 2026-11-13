import assert from 'node:assert/strict'
import { prepareSquareImage } from '../app/utils/prepare-square-image.ts'
import { prepareAvatarImage } from '../app/utils/avatar-image.ts'
import { createNetworkForm } from '../app/composables/network/useNetworkForm.ts'
import { maxIconBytes } from '../shared/preferences.ts'

const originals = new Map()
function replaceGlobal(name, value) {
  if (!originals.has(name)) originals.set(name, Object.getOwnPropertyDescriptor(globalThis, name))
  Object.defineProperty(globalThis, name, { configurable: true, writable: true, value })
}

const encodedImage = 'data:image/webp;base64,cropped'
function imageFixture({ width = 400, height = 200, stage, failure, url = encodedImage } = {}) {
  const calls = { decoded: [], closed: 0, draw: [], encode: [] }
  const bitmap = { width, height, close: () => calls.closed++ }
  const canvas = {
    width: 0,
    height: 0,
    getContext(type) {
      assert.equal(type, '2d')
      if (stage === 'context') throw failure
      if (stage === 'missing-context') return null
      return {
        drawImage(...args) {
          if (stage === 'draw') throw failure
          calls.draw.push(args)
        },
      }
    },
    toDataURL(...args) {
      if (stage === 'encode') throw failure
      calls.encode.push(args)
      return url
    },
  }
  replaceGlobal('createImageBitmap', async (file) => {
    calls.decoded.push(file)
    if (stage === 'decode') throw failure
    return bitmap
  })
  replaceGlobal('document', {
    createElement(tag) {
      assert.equal(tag, 'canvas')
      if (stage === 'canvas') throw failure
      return canvas
    },
  })
  return { calls, bitmap, canvas }
}

const file = (type, bytes = 1) => new File([new Uint8Array(bytes)], 'chosen-image', { type })
const translate = (key) => `translated:${key}`

try {
  for (const [width, height, size, crop] of [
    [400, 200, 256, [100, 0, 200, 200]],
    [200, 401, 96, [0, 100.5, 200, 200]],
    [120, 120, 96, [0, 0, 120, 120]],
  ]) {
    const fixture = imageFixture({ width, height })
    const chosen = file('image/png')
    assert.equal(await prepareSquareImage(chosen, size), encodedImage)
    assert.deepEqual(fixture.calls.decoded, [chosen])
    assert.deepEqual(fixture.calls.draw, [[fixture.bitmap, ...crop, 0, 0, size, size]])
    assert.equal(fixture.canvas.width, size)
    assert.equal(fixture.canvas.height, size)
    assert.deepEqual(fixture.calls.encode, [['image/webp', 0.85]])
    assert.equal(fixture.calls.closed, 1)
  }

  for (const stage of ['decode', 'canvas', 'context', 'draw', 'encode']) {
    const failure = new Error(`${stage} failed`)
    const fixture = imageFixture({ stage, failure })
    await assert.rejects(prepareSquareImage(file('image/png'), 256), (error) => error === failure)
    assert.equal(fixture.calls.closed, stage === 'decode' ? 0 : 1, stage)
  }

  const missingContext = imageFixture({ stage: 'missing-context' })
  await assert.rejects(prepareSquareImage(file('image/png'), 96), TypeError)
  assert.equal(missingContext.calls.closed, 1)

  for (const type of ['image/png', 'image/jpeg', 'image/webp']) {
    const fixture = imageFixture()
    assert.equal(await prepareAvatarImage(file(type)), encodedImage)
    assert.equal(fixture.canvas.width, 256)
    assert.equal(fixture.calls.closed, 1)
  }
  const invalidAvatar = imageFixture()
  await assert.rejects(prepareAvatarImage(file('image/gif')), {
    message: 'Choose a PNG, JPEG or WebP image.',
  })
  await assert.rejects(prepareAvatarImage(file('image/svg+xml'), translate), {
    message: 'translated:mediaUi.avatarImage.chooseFormat',
  })
  assert.equal(invalidAvatar.calls.decoded.length, 0)

  const avatarLimit = 5 * 1024 * 1024
  const exactAvatar = imageFixture()
  await prepareAvatarImage(file('image/png', avatarLimit))
  assert.equal(exactAvatar.calls.closed, 1)
  const oversizedAvatar = imageFixture()
  const translations = []
  await assert.rejects(
    prepareAvatarImage(
      file('image/png', avatarLimit + 1),
      (key, values) => {
        translations.push({ key, values })
        return translate(key)
      },
      'ar',
    ),
    { message: 'translated:mediaUi.avatarImage.tooLarge' },
  )
  assert.deepEqual(translations, [
    {
      key: 'mediaUi.avatarImage.tooLarge',
      values: { size: `${new Intl.NumberFormat('ar').format(5)} MB` },
    },
  ])
  assert.equal(oversizedAvatar.calls.decoded.length, 0)

  const avatarContext = imageFixture({ stage: 'missing-context' })
  await assert.rejects(prepareAvatarImage(file('image/png'), translate), {
    message: 'translated:mediaUi.avatarImage.prepareFailed',
  })
  assert.equal(avatarContext.calls.closed, 1)
  for (const stage of ['decode', 'draw', 'encode']) {
    const failure = new Error(`${stage} avatar failed`)
    const fixture = imageFixture({ stage, failure })
    await assert.rejects(prepareAvatarImage(file('image/png')), (error) => error === failure)
    assert.equal(fixture.calls.closed, stage === 'decode' ? 0 : 1)
  }

  // Isolate icon selection from form lifecycle, stores, timers, and network requests.
  const globals = {
    useI18n: () => ({ t: translate }),
    useChatStore: () => ({ data: { networks: [], user: { nick: 'tester' } } }),
    useAuthStore: () => ({ status: 'signed-in', account: { id: 'tester' } }),
    usePreferencesStore: () => ({ preferences: { networkIcons: {} } }),
    usePrompt: () => ({ confirm: async () => false }),
    useRuntimeConfig: () => ({ public: { networkPresets: [] } }),
    ref: (value) => ({ value }),
    reactive: (value) => value,
    computed: (get) => ({
      get value() {
        return get()
      },
    }),
    watch: () => {},
    watchEffect: () => {},
    onMounted: () => {},
    provide: () => {},
  }
  for (const [name, value] of Object.entries(globals)) replaceGlobal(name, value)
  const form = createNetworkForm(
    { networkId: 'draft', prefill: {}, firstConnection: false },
    () => {},
  )
  const choose = async (chosen) => {
    const target = { files: chosen ? [chosen] : [], value: 'selected-file' }
    await form.chooseIcon({ target })
    assert.equal(target.value, '', 'selection is cleared so the same file can be chosen again')
  }

  const networkGif = imageFixture({ height: 400, width: 200 })
  await choose(file('image/gif', avatarLimit + 1))
  assert.equal(form.icon.value, encodedImage)
  assert.equal(networkGif.canvas.width, 96)
  assert.equal(networkGif.canvas.height, 96)
  assert.deepEqual(networkGif.calls.draw[0].slice(1), [0, 100, 200, 200, 0, 0, 96, 96])
  assert.deepEqual(networkGif.calls.encode, [['image/webp', 0.85]])
  assert.equal(networkGif.calls.closed, 1)

  const exactIcon = encodedImage.padEnd(maxIconBytes, 'x')
  const networkLimit = imageFixture({ url: exactIcon })
  await choose(file('image/png'))
  assert.equal(form.icon.value, exactIcon)
  assert.equal(networkLimit.calls.closed, 1)
  const oversizedIcon = imageFixture({ url: `${exactIcon}x` })
  await choose(file('image/png'))
  assert.equal(form.icon.value, exactIcon, 'an oversized result preserves the previous icon')
  assert.equal(form.error.value, 'translated:networkForm.imageTooDetailed')
  assert.equal(oversizedIcon.calls.closed, 1)

  for (const stage of ['decode', 'canvas', 'context', 'draw', 'encode']) {
    const failure = new Error(`${stage} network failed`)
    const fixture = imageFixture({ stage, failure })
    await choose(file('image/png'))
    assert.equal(form.error.value, failure.message)
    assert.equal(form.icon.value, exactIcon)
    assert.equal(fixture.calls.closed, stage === 'decode' ? 0 : 1, stage)
  }
  const networkContext = imageFixture({ stage: 'missing-context' })
  await choose(file('image/png'))
  assert(form.error.value, 'missing context preserves the browser error message')
  assert.equal(networkContext.calls.closed, 1)
  const networkUnknown = imageFixture({ stage: 'decode', failure: 'not an Error' })
  await choose(file('image/png'))
  assert.equal(form.error.value, 'translated:networkForm.imageOpenFailed')
  assert.equal(networkUnknown.calls.closed, 0)
  const noSelection = imageFixture()
  await choose()
  assert.equal(noSelection.calls.decoded.length, 0)
  assert.equal(form.icon.value, exactIcon)

  console.log(
    'Square image preparation: crops, encoding, avatar/network policies, and bitmap cleanup passed.',
  )
} finally {
  for (const [name, descriptor] of originals) {
    if (descriptor) Object.defineProperty(globalThis, name, descriptor)
    else delete globalThis[name]
  }
}
