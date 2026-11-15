// Read-aloud's voice model on this device (#204): ZONOS3-Mini running on WebGPU in the browser,
// as an alternative to a speech service. The engine and its small files are vendored under
// public/speech/zonos3/<version>/ (scripts/vendor-local-speech.mjs); the weight shards come from
// Hugging Face at a pinned revision, through a redirect (server/routes/speech/[...path].ts).
// Shared by the browser, the redirect route and the page's Content-Security-Policy.
import { localSpeechModel, localSpeechVoices } from './generated/local-speech-model'

export { localSpeechModel, localSpeechVoices }

/** Per-device switch and voice, next to read-aloud's own switch; both cleared on sign-out. */
export const localSpeechStorage = 'ircord.device.localSpeech'
export const localVoiceStorage = 'ircord.device.localVoice'
/** Whether this device already showed the downsides, so they open by themselves only once. */
export const localSpeechInfoStorage = 'ircord.device.localSpeechInfo'
/** The last measured speed (generation time ÷ audio time) on this device. */
export const localSpeechSpeedStorage = 'ircord.device.localSpeechSpeed'

/** The hosts the browser downloads the shards from: the Hub, then its CDN. */
export const localSpeechHosts = ['https://huggingface.co', 'https://*.hf.co'] as const
/** Cache Storage the engine keeps the shards in (src/gpu/weights.ts CACHE_NAME upstream). */
export const localSpeechCache = 'zonos3-web-models-v6'
export const localSpeechCachePrefix = 'zonos3-web-models-'
/** The engine's files on this origin. */
export const localSpeechBase = `/speech/zonos3/${localSpeechModel.version}/`
export const localSpeechModelsPath = `${localSpeechBase}models/`

/** Rupert, English (British), male. */
export const defaultLocalVoice = 'default_3765f0d1140c05c0'
export type LocalVoice = (typeof localSpeechVoices)[number]

export function sanitizeLocalVoice(value: unknown) {
  return localSpeechVoices.some((v) => v.id === value) ? (value as string) : defaultLocalVoice
}
export const localVoice = (id: string): LocalVoice =>
  localSpeechVoices.find((v) => v.id === id) ??
  localSpeechVoices.find((v) => v.id === defaultLocalVoice)!

/** The voices as select options, grouped by language in upstream's order. */
export function localVoiceOptions(
  translate: (key: string, values?: Record<string, string>) => string,
) {
  return localSpeechVoices.map((v) => {
    const values = {
      name: v.name,
      language: v.group,
      style: v.style,
      gender: translate(`readAloud.local.gender.${v.gender}`),
    }
    return {
      value: v.id,
      group: v.group,
      label: translate(
        v.style ? 'readAloud.local.voiceOption' : 'readAloud.local.voiceOptionPlain',
        values,
      ),
    }
  })
}

/**
 * IRCord's spoken line, prepared for the model. The browser engine has no text normalization of
 * its own, and reads `#dev` as noise, `@alice` as "at alice" and `xX_dark_Xx` with "underscore"s.
 * Numbers, times and CamelCase nicknames read well as they are.
 */
export function prepareLocalSpeech(line: string) {
  return (
    line
      // Mentions and "&" channels: "@alice" is said "alice". The "#" of "#dev" goes below.
      .replace(/(^|[\s(])[&@]+(?=[\p{L}\p{N}_])/gu, '$1')
      // Dashes inside names: "dev-ops-2" reads as words. Dates such as 2026-11-20 stay.
      .replace(/(?<=\p{L})-(?=[\p{L}\p{N}])|(?<=\p{N})-(?=\p{L})/gu, ' ')
      // Emoji and symbols the model stumbles on, including "#" and the "_" of "xX_dark_Xx".
      .replace(/[\p{Extended_Pictographic}\u{FE0F}\u{200D}]/gu, ' ')
      .replace(/[*~^`<>{}[\]\\=_|#]+/g, ' ')
      // Repeated punctuation: "???" is one question, "!!!" one exclamation.
      .replace(/([!?])[!?]+/g, '$1')
      .replace(/\.{4,}/g, '…')
      .replace(/([,;:])\1+/g, '$1')
      .replace(/\s+([,;:!?.…)])/g, '$1')
      .replace(/\(\s+/g, '(')
      .replace(/\s+/g, ' ')
      .trim()
  )
}

/**
 * The engine's /v2/generate request for one line: the voice's shipped embedding (faster to first
 * audio than its reference codes), the voice's language, streamed float32 PCM, and alignment
 * guarding against skipped words. `short_text: 'off'` keeps upstream's one- and two-word path from
 * swallowing the sender of "alice: ok".
 */
export function localSpeechRequest(text: string, voiceId: string) {
  const voice = localVoice(voiceId)
  return {
    text: prepareLocalSpeech(text),
    language: voice.language,
    speaker_mode: 'embedding',
    speaker_embedding_id: voice.id,
    stream: true,
    format: 'pcm',
    laci: true,
    short_text: 'off',
  }
}
/** The engine's output: float32 mono at this rate. */
export const localSpeechSampleRate = 44_100

/**
 * The shard URL on the Hub for a path under /speech/, or '' when it is not one of the shipped
 * manifests' shards. The redirect route serves only these, so the server never becomes an open
 * redirect or a proxy for other files.
 */
export function localSpeechShardUrl(path: string) {
  const match = /^zonos3\/([a-z0-9]+)\/models\/([a-z_]+)\/(shard-\d{3}\.bin)$/.exec(path)
  if (!match || match[1] !== localSpeechModel.version) return ''
  const [, , model, name] = match
  if (!localSpeechModel.shards[model!]?.some((s) => s.name === name)) return ''
  return `https://huggingface.co/${localSpeechModel.hubRepo}/resolve/${localSpeechModel.hubRevision}/web/${model}/${name}`
}

/** "921 MB": sizes as people read them on a download, in decimal megabytes. */
export function formatDownloadSize(bytes: number) {
  if (bytes >= 1e9) return `${(bytes / 1e9).toFixed(1)} GB`
  return `${Math.max(0, Math.round(bytes / 1e6))} MB`
}

/**
 * Phones and tablets are left out at first: a memory failure there kills the whole chat tab,
 * and no real device was measured. Besides the phone and tablet names, a computer must have a
 * precise pointer that can hover (a mouse or trackpad): Android tablets in desktop-site or
 * desktop mode, and iPadOS, ask for desktop pages and name no tablet. A touch laptop still
 * counts as a computer, through its trackpad.
 */
export function mobileDevice(device: {
  userAgent: string
  mobileHint?: boolean
  maxTouchPoints?: number
  /** `(any-pointer: fine)`: some pointer is precise. */
  finePointer: boolean
  /** `(any-hover: hover)`: some pointer can hover. */
  hover: boolean
}) {
  if (device.mobileHint) return true
  if (/Android|iPhone|iPad|iPod|Mobile|Tablet|Silk|Kindle/i.test(device.userAgent)) return true
  // iPadOS reports itself as a Mac with a touch screen.
  if (/Macintosh/.test(device.userAgent) && (device.maxTouchPoints ?? 0) > 1) return true
  return !(device.finePointer && device.hover)
}

export type LocalSpeechSupport = 'available' | 'off' | 'mobile' | 'no-webgpu'
/**
 * Whether this device can run the model. A software adapter (SwiftShader) runs it about 11 times
 * slower than real time, so it counts as no WebGPU.
 */
export function localSpeechSupport(device: {
  enabled: boolean
  mobile: boolean
  gpu: boolean
  adapter?: { isFallbackAdapter?: boolean } | null
}): LocalSpeechSupport {
  if (!device.enabled) return 'off'
  if (device.mobile) return 'mobile'
  if (!device.gpu || !device.adapter || device.adapter.isFallbackAdapter) return 'no-webgpu'
  return 'available'
}

/**
 * How to play on this device, from its speed (generation time ÷ audio time): stream as it
 * arrives, buffer the whole line first to avoid gaps, or stop using the model. Above 0.9 a
 * stream has too little margin to play without gaps.
 */
export function localSpeechPace(speed: number | null) {
  if (speed === null || speed <= 0.7) return 'stream'
  return speed <= 0.9 ? 'buffer' : 'slow'
}
/** Speeds of the last few lines; one line read while the GPU was busy must not decide. */
export const speedSamples = 3
const validSpeed = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value) && value > 0 && value < 100
export const sanitizeSpeeds = (value: unknown) =>
  Array.isArray(value) ? value.filter(validSpeed).slice(-speedSamples) : []
/**
 * This device's speed: the median of its recent lines, or the faster of two. A single slow line
 * among faster ones does not mark the device slow.
 */
export function deviceSpeed(speeds: readonly number[]) {
  if (!speeds.length) return null
  const sorted = [...speeds].sort((a, b) => a - b)
  return sorted[Math.floor((sorted.length - 1) / 2)]!
}

/** The page policy's hosts for the shard download, or none when the operator switched it off. */
export const localSpeechConnectSources = (enabled: boolean) =>
  enabled ? localSpeechHosts.join(' ') : ''
