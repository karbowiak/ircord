import {
  defaultLocalVoice,
  formatDownloadSize,
  localSpeechCache,
  localSpeechCachePrefix,
  localSpeechInfoStorage,
  localSpeechModel,
  localSpeechModelsPath,
  localSpeechPace,
  localSpeechRequest,
  localSpeechSampleRate,
  localSpeechSpeedStorage,
  localSpeechStorage,
  localSpeechSupport,
  localVoiceStorage,
  mobileDevice,
  sanitizeLocalVoice,
  sanitizeSpeeds,
  deviceSpeed,
  speedSamples,
  type LocalSpeechSupport,
} from '../../../shared/local-speech'
import { LocalSpeechEngine, type LocalSpeechProgress } from '../../utils/local-speech-engine'
import { PcmPlayer } from '../../utils/pcm-player'

/** How much of the model this browser keeps: none, some shards (an interrupted download) or all. */
export type LocalModelStored = 'unknown' | 'absent' | 'partial' | 'stored'

/** A failure to tell the user about, already in their words. */
export class LocalSpeechError extends Error {
  constructor(
    message: string,
    readonly kind: 'setup' | 'unavailable' | 'slow' | 'failed' = 'failed',
  ) {
    super(message)
    this.name = 'LocalSpeechError'
  }
}

/** Unload after this long without a line, so an idle tab gives the memory back. */
const idleUnload = 30 * 60_000
/** After a Test or a download with read-aloud off, keep the model briefly for another Test. */
const shortUnload = 2 * 60_000
/** One tab holds the model: the others leave highlights to it. */
export const localSpeechLock = 'ircord.localSpeech.engine'

const on = ref(readLocal(localSpeechStorage, (v) => v === true))
const voice = ref(readLocal(localVoiceStorage, sanitizeLocalVoice))
const infoSeen = ref(readLocal(localSpeechInfoStorage, (v) => v === true))
const speeds = ref(readLocal(localSpeechSpeedStorage, sanitizeSpeeds))
const speed = computed(() => deviceSpeed(speeds.value))
const support = ref<LocalSpeechSupport | 'checking'>('checking')
const stored = ref<LocalModelStored>('unknown')
const storedBytes = ref(0)
/** idle: nothing in memory; loading: downloading or starting; ready: on the GPU. */
const phase = ref<'idle' | 'loading' | 'ready'>('idle')
const progress = ref<(LocalSpeechProgress & { rate: number }) | null>(null)
/** What went wrong with the last download or start, in the engine's words. */
const failure = ref('')

let engine: LocalSpeechEngine | undefined
let starting: Promise<LocalSpeechEngine> | undefined
let idleTimer: ReturnType<typeof setTimeout> | undefined
let releaseLock: (() => void) | undefined
let context: AudioContext | undefined
let supportCheck: Promise<void> | undefined
let fatalNotice: ((message: string) => void) | undefined
/** Lines being generated now; a failure during one is reported through that line. */
let linesInFlight = 0
/** Bumped by every unload, so a load still in its checks knows it was cancelled. */
let loadSeq = 0
/** This tab loaded the model while another tab held it; see keepFor. */
let guest = false
/** The operator can switch the feature off (NUXT_PUBLIC_LOCAL_SPEECH=false). */
let operatorOn = true

const shardUrls = () =>
  Object.entries(localSpeechModel.shards).flatMap(([model, shards]) =>
    shards.map((s) => ({
      url: new URL(`${localSpeechModelsPath}${model}/${s.name}`, location.href).href,
      size: s.size,
    })),
  )

/** Whether this device can run the model; asks the browser for a graphics adapter once. */
function checkSupport() {
  return (supportCheck ??= (async () => {
    const enabled = operatorOn
    const nav = navigator as Navigator & {
      userAgentData?: { mobile?: boolean }
      gpu?: {
        requestAdapter(
          options?: object,
        ): Promise<{ info?: { isFallbackAdapter?: boolean }; isFallbackAdapter?: boolean } | null>
      }
    }
    const media = (query: string) => !!window.matchMedia?.(query).matches
    const mobile = mobileDevice({
      userAgent: nav.userAgent,
      mobileHint: nav.userAgentData?.mobile,
      maxTouchPoints: nav.maxTouchPoints,
      finePointer: media('(any-pointer: fine)'),
      hover: media('(any-hover: hover)'),
    })
    let adapter: { isFallbackAdapter?: boolean } | null = null
    if (enabled && !mobile && nav.gpu)
      try {
        const found = await nav.gpu.requestAdapter({ powerPreference: 'high-performance' })
        adapter = found
          ? { isFallbackAdapter: found.info?.isFallbackAdapter ?? found.isFallbackAdapter ?? false }
          : null
      } catch {
        adapter = null
      }
    support.value = localSpeechSupport({ enabled, mobile, gpu: !!nav.gpu, adapter })
  })())
}

/** Which shards this browser's Cache Storage holds. Opens no cache that does not exist yet. */
async function checkStored() {
  try {
    if (typeof caches === 'undefined' || !(await caches.has(localSpeechCache))) {
      stored.value = 'absent'
      storedBytes.value = 0
      return
    }
    const cache = await caches.open(localSpeechCache)
    let bytes = 0
    let count = 0
    const shards = shardUrls()
    for (const shard of shards)
      if (await cache.match(shard.url)) {
        bytes += shard.size
        count++
      }
    storedBytes.value = bytes
    stored.value = count === shards.length ? 'stored' : count ? 'partial' : 'absent'
  } catch {
    stored.value = 'absent'
  }
}

/** Free space, when the browser can tell; the download needs the model's size. */
async function spaceLeft() {
  try {
    const estimate = await navigator.storage?.estimate()
    if (estimate?.quota === undefined) return Infinity
    return estimate.quota - (estimate.usage ?? 0)
  } catch {
    return Infinity
  }
}

function scheduleUnload(after: number) {
  clearTimeout(idleTimer)
  idleTimer = setTimeout(() => unload(), after)
}

/** Stops both workers and frees the model's memory. The shards stay in the browser's cache. */
function unload() {
  // A load still in its checks sees this and stops.
  loadSeq++
  clearTimeout(idleTimer)
  idleTimer = undefined
  const current = engine
  engine = undefined
  starting = undefined
  guest = false
  current?.terminate()
  releaseLock?.()
  releaseLock = undefined
  if (phase.value !== 'idle') phase.value = 'idle'
  progress.value = null
}

/**
 * Takes the model lock for as long as the engine lives, so other tabs leave highlights to this
 * one. False when another tab already holds it: this tab's engine is then only a guest.
 */
async function holdLock() {
  if (releaseLock || !navigator.locks) return true
  return new Promise<boolean>((granted) => {
    void navigator.locks.request(localSpeechLock, { ifAvailable: true }, (lock) => {
      granted(!!lock)
      return lock ? new Promise<void>((resolve) => (releaseLock = resolve)) : undefined
    })
  })
}

/**
 * When this tab may keep the model loaded after a line, Test or Download. A guest (another tab
 * holds the model and reads the highlights) unloads at once, so two copies never stay in memory.
 */
const keepFor = (readAloudOn: boolean) => (guest ? 0 : readAloudOn ? idleUnload : shortUnload)

/** Whether another tab of this browser has the model loaded. */
export async function otherTabHoldsModel() {
  if (releaseLock || !navigator.locks?.query) return false
  try {
    const { held } = await navigator.locks.query()
    return !!held?.some((lock) => lock.name === localSpeechLock)
  } catch {
    return false
  }
}

/**
 * Loads the model onto the GPU. Downloads only when `download` is set, which only a click in
 * Settings does: a highlight never starts a 921 MB download.
 */
function load(
  t: (key: string, values?: Record<string, string>) => string,
  download: boolean,
): Promise<LocalSpeechEngine> {
  if (engine && phase.value === 'ready') return Promise.resolve(engine)
  // One load at a time, checks included: a Test and a highlight never start two engines.
  const current = (starting ??= start(t, download))
  current.catch(() => {
    if (starting === current) starting = undefined
  })
  return current
}

async function start(
  t: (key: string, values?: Record<string, string>) => string,
  download: boolean,
) {
  // Switching off while this waits on the checks below cancels the load.
  const token = loadSeq
  const current = () => {
    if (token === loadSeq) return
    throw new DOMException('The operation was aborted.', 'AbortError')
  }
  await checkSupport()
  current()
  if (support.value !== 'available')
    throw new LocalSpeechError(t('workspaceUi.speech.localUnavailable'), 'unavailable')
  await checkStored()
  current()
  if (stored.value !== 'stored') {
    if (!download) throw new LocalSpeechError(t('workspaceUi.speech.localNotDownloaded'), 'setup')
    const needed = localSpeechModel.downloadBytes - storedBytes.value
    const free = await spaceLeft()
    current()
    if (free < needed * 1.05)
      throw new LocalSpeechError(
        t('readAloud.local.noSpace', {
          free: formatDownloadSize(free),
          size: formatDownloadSize(needed),
        }),
        'failed',
      )
    // Ask the browser not to evict the model under storage pressure; it may say no.
    void navigator.storage?.persist?.().catch(() => false)
  }
  const owner = await holdLock()
  if (token !== loadSeq) {
    // Switched off while waiting for the lock: give it back.
    if (owner) {
      releaseLock?.()
      releaseLock = undefined
    }
    throw new DOMException('The operation was aborted.', 'AbortError')
  }
  guest = !owner
  failure.value = ''
  phase.value = 'loading'
  progress.value = null
  let last = { at: performance.now(), fetched: 0 }
  const created = new LocalSpeechEngine(
    (p) => {
      const now = performance.now()
      const seconds = (now - last.at) / 1000
      // A smoothed network rate, for "about 30 s left".
      const instant = seconds > 0.25 ? (p.fetched - last.fetched) / seconds : NaN
      const previous = progress.value?.rate ?? 0
      const rate = Number.isNaN(instant)
        ? previous
        : previous
          ? previous * 0.7 + instant * 0.3
          : instant
      if (seconds > 0.25) last = { at: now, fetched: p.fetched }
      progress.value = { ...p, rate }
    },
    (message) => {
      // A lost GPU device or a crashed worker: free everything, load again for the next line.
      if (engine === created) unload()
      failure.value = message
      // A line being read reports it itself.
      if (!linesInFlight) fatalNotice?.(message)
    },
  )
  engine = created
  try {
    await created.ready
  } catch (error) {
    void checkStored()
    // Cancel, Remove or switching off unloaded it on purpose.
    if (engine !== created) throw new DOMException('The operation was aborted.', 'AbortError')
    unload()
    const message = (error as Error)?.message ?? String(error)
    failure.value = message
    throw new LocalSpeechError(t('readAloud.local.startFailed', { message }), 'failed')
  }
  if (engine !== created) throw new DOMException('The operation was aborted.', 'AbortError')
  starting = undefined
  phase.value = 'ready'
  progress.value = null
  void checkStored()
  return created
}

async function audioContext() {
  context ??= new AudioContext()
  if (context.state !== 'running') {
    // Without a click or key press first, the browser keeps the context suspended.
    await Promise.race([context.resume(), new Promise((resolve) => setTimeout(resolve, 300))])
    if ((context.state as string) !== 'running')
      throw new DOMException('Audio playback needs a user gesture first.', 'NotAllowedError')
  }
  return context
}

/**
 * The voice model on this device (#204): the switch, the voice, and the model's download, memory
 * and speech. Speaking streams the engine's PCM into a scheduled player, and measures this
 * device's speed so a slow one buffers each line or stops using the model.
 */
export function useLocalSpeech() {
  const { t } = useI18n()
  const translate = (key: string, values?: Record<string, string>) =>
    values ? t(key, values) : t(key)
  fatalNotice ??= (message) =>
    useChatStore().notify(t('workspaceUi.speech.localFailed', { error: message }))
  operatorOn = useRuntimeConfig().public.localSpeech !== false
  /** The switch counts only while the operator offers the feature. */
  const active = computed(() => on.value && operatorOn)

  function setOn(value: boolean) {
    on.value = value
    writeLocal(localSpeechStorage, value)
    if (!value) unload()
  }
  function setVoice(value: string) {
    voice.value = sanitizeLocalVoice(value)
    writeLocal(localVoiceStorage, voice.value)
  }
  function markInfoSeen() {
    infoSeen.value = true
    writeLocal(localSpeechInfoStorage, true)
  }
  function setSpeeds(value: number[]) {
    speeds.value = value.slice(-speedSamples)
    writeLocal(localSpeechSpeedStorage, speeds.value)
  }

  /** The Download button: downloads, loads to check the model runs, then frees it if unused. */
  async function download(readAloudOn: boolean) {
    await load(translate, true)
    scheduleUnload(keepFor(readAloudOn))
  }
  function cancel() {
    unload()
    void checkStored()
  }
  /** Deletes the model from this browser. */
  async function remove() {
    unload()
    try {
      for (const name of await caches.keys())
        if (name.startsWith(localSpeechCachePrefix)) await caches.delete(name)
    } catch {
      /* Nothing to delete. */
    }
    failure.value = ''
    setSpeeds([])
    await checkStored()
  }

  /**
   * Speaks one line. `test` may download (it is a click), measures afresh even on a device
   * marked slow, and frees the model soon after when read-aloud is off.
   */
  async function speak(
    text: string,
    signal: AbortSignal,
    options: { test?: boolean; readAloudOn: boolean },
  ) {
    const pace = localSpeechPace(speed.value)
    if (pace === 'slow' && !options.test)
      throw new LocalSpeechError(t('workspaceUi.speech.localSlow'), 'slow')
    const current = await load(translate, !!options.test)
    clearTimeout(idleTimer)
    let stopPlayer: (() => void) | undefined
    try {
      const ctx = await audioContext()
      const player = new PcmPlayer(ctx, localSpeechSampleRate, { buffered: pace === 'buffer' })
      stopPlayer = () => player.stop()
      signal.addEventListener('abort', stopPlayer, { once: true })
      const started = performance.now()
      linesInFlight++
      await current
        .generate(localSpeechRequest(text, voice.value), signal, (pcm) => player.push(pcm))
        .finally(() => linesInFlight--)
      const took = (performance.now() - started) / 1000
      // Short lines carry the model's fixed costs; judge speed on a second or more of audio. A
      // Test measures afresh; highlights add to the recent lines.
      if (player.seconds >= 1) {
        const measured = took / player.seconds
        setSpeeds(options.test ? [measured] : [...speeds.value, measured])
      }
      if (!signal.aborted) await player.end()
    } finally {
      if (stopPlayer) signal.removeEventListener('abort', stopPlayer)
      // Kept for the next line, then freed when idle; a guest tab frees it at once.
      if (engine === current) scheduleUnload(keepFor(options.readAloudOn))
    }
    if (localSpeechPace(speed.value) === 'slow')
      throw new LocalSpeechError(t('workspaceUi.speech.localSlow'), 'slow')
  }

  /** Settings and the top bar need these facts; checking them loads nothing. */
  function refresh() {
    if (!import.meta.client) return
    void checkSupport()
    void checkStored()
  }

  const ready = computed(
    () =>
      support.value === 'available' &&
      stored.value === 'stored' &&
      localSpeechPace(speed.value) !== 'slow',
  )
  return {
    on,
    active,
    operatorOn,
    voice,
    infoSeen,
    speed,
    support,
    stored,
    storedBytes,
    phase,
    progress,
    failure,
    ready,
    setOn,
    setVoice,
    markInfoSeen,
    download,
    cancel,
    remove,
    speak,
    refresh,
    unload,
    defaultVoice: defaultLocalVoice,
  }
}

// The switch and voice are per device: a change in another tab of this browser applies here too.
if (import.meta.client)
  window.addEventListener('storage', (event) => {
    if (event.key === localSpeechStorage) {
      on.value = event.newValue === 'true'
      if (!on.value) unload()
    } else if (event.key === localVoiceStorage) {
      let value: unknown
      try {
        value = JSON.parse(event.newValue ?? 'null')
      } catch {
        /* Not ours. */
      }
      voice.value = sanitizeLocalVoice(value)
    } else if (event.key === localSpeechSpeedStorage) {
      try {
        speeds.value = sanitizeSpeeds(JSON.parse(event.newValue ?? 'null'))
      } catch {
        speeds.value = []
      }
    }
  })
