<script setup lang="ts">
import { gifFetchHosts } from '../../shared/utils/gif-hosts'

/**
 * An image that holds a still frame unless `playing`. Browsers cannot pause an animated <img>,
 * and drawing one onto a canvas always yields its first frame, so this component plays the
 * animation itself: when playing starts it reads the image's bytes, decodes them frame by frame
 * with WebCodecs' ImageDecoder and paints each frame onto a canvas. Stopping leaves the canvas
 * on the frame it reached, and playing again carries on from there.
 *
 * Only IRCord's own origins and the hosts in gifFetchHosts (which the CSP's connect-src allows)
 * are read. For any other host, when the bytes can't be read or decoded, or when the browser has
 * no ImageDecoder, the <img> animates while playing instead, and stopping shows the first frame.
 *
 * Memory stays bounded in long timelines: a decoder and its bytes exist only while playing;
 * a stopped image keeps just one size-capped canvas and the position it reached.
 */
const props = defineProps<{
  src: string
  playing: boolean
  alt?: string
  width?: number
  height?: number
  /** Images from other sites load lazily and without a referrer. */
  external?: boolean
}>()
const emit = defineEmits<{ load: []; error: [] }>()
const canvas = ref<HTMLCanvasElement>()
const image = ref<HTMLImageElement>()
const loaded = ref(false)
/** The <img> animates while playing because this image can't be decoded here. */
const native = ref(typeof ImageDecoder === 'undefined')
/** Caps the still's backing store; it is shown at most 400 CSS pixels wide. */
const maxStill = 960

/** Where playback stopped: the frame on the canvas, time left on it, and loops completed. */
let frame = 0
let remaining = 0
let loops = 0
/** Each play gets a new session; work from an older one stops when it notices. */
let session = 0
let decoder: ImageDecoder | undefined
let timer: ReturnType<typeof setTimeout> | undefined
let due = 0

function paint(source: CanvasImageSource, naturalWidth: number, naturalHeight: number) {
  const target = canvas.value
  if (!target || !naturalWidth || !naturalHeight) return
  const scale = Math.min(1, maxStill / Math.max(naturalWidth, naturalHeight))
  const width = Math.round(naturalWidth * scale)
  const height = Math.round(naturalHeight * scale)
  if (target.width !== width || target.height !== height) {
    target.width = width
    target.height = height
  }
  const context = target.getContext('2d')
  context?.clearRect(0, 0, width, height)
  context?.drawImage(source, 0, 0, width, height)
}
/** Copies the <img> onto the canvas; for an animation that is its first frame. */
function capture() {
  const source = image.value
  if (source) paint(source, source.naturalWidth, source.naturalHeight)
}
function still() {
  if (!frame) capture()
  loaded.value = true
  emit('load')
}

/** Browsers stretch near-zero GIF delays to 100ms; match them so speeds agree. */
const delay = (microseconds: number | null) => {
  const ms = (microseconds ?? 0) / 1000
  return ms <= 10 ? 100 : ms
}

/**
 * Origins the page may read bytes from: the listed GIF hosts and IRCord's own origins (uploads).
 * Asking first keeps unlisted hosts from tripping the CSP and logging errors on every hover.
 */
const readable = () => {
  try {
    const apiBase = useRuntimeConfig().public.apiBase as string
    const own = [location.origin, new URL(apiBase || '/', location.href).origin]
    const origin = new URL(props.src, location.href).origin
    return own.includes(origin) || gifFetchHosts.includes(origin)
  } catch {
    return false
  }
}
/** Gives up decoding this image; the <img> animates while playing from now on. */
function fail(id: number) {
  if (id !== session) return
  stop()
  native.value = true
}

async function start() {
  const id = ++session
  if (native.value) return
  if (!readable()) {
    native.value = true
    return
  }
  try {
    const response = await fetch(props.src, {
      mode: 'cors',
      credentials: 'omit',
      referrerPolicy: props.external ? 'no-referrer' : undefined,
    })
    const type = response.headers.get('content-type')?.split(';')[0]?.trim() || 'image/gif'
    if (!response.ok || !(await ImageDecoder.isTypeSupported(type))) throw new Error(type)
    const data = await response.arrayBuffer()
    if (id !== session) return
    const next = new ImageDecoder({ data, type })
    await next.tracks.ready
    if (id !== session) return next.close()
    decoder = next
    await show(id)
  } catch {
    fail(id)
  }
}
async function show(id: number) {
  const current = decoder
  if (!current || id !== session) return
  const track = current.tracks.selectedTrack
  if (frame >= (track?.frameCount ?? 1)) frame = 0
  const { image: decoded } = await current.decode({ frameIndex: frame })
  if (id !== session) return decoded.close()
  paint(decoded, decoded.displayWidth, decoded.displayHeight)
  const wait = remaining || delay(decoded.duration)
  decoded.close()
  remaining = 0
  if ((track?.frameCount ?? 1) < 2) return
  due = performance.now() + wait
  // A frame that fails to decode (e.g. a truncated file) hands playback to the <img>.
  timer = setTimeout(() => void advance(id).catch(() => fail(id)), wait)
}
async function advance(id: number) {
  const track = decoder?.tracks.selectedTrack
  if (!track || id !== session) return
  if (frame + 1 < track.frameCount) frame++
  else if (++loops <= track.repetitionCount) frame = 0
  else return // A GIF with a set number of loops rests on its last frame.
  await show(id)
}
/** Ends this play, keeping the frame on the canvas and the time it had left. */
function stop() {
  session++
  if (timer !== undefined) {
    clearTimeout(timer)
    timer = undefined
    remaining = Math.max(0, due - performance.now())
  }
  decoder?.close()
  decoder = undefined
}

watch(
  () => props.playing,
  (playing) => {
    if (playing) void start()
    else stop()
  },
  { immediate: true },
)
watch(
  () => props.src,
  () => {
    stop()
    loaded.value = false
    native.value = typeof ImageDecoder === 'undefined'
    frame = remaining = loops = 0
    if (props.playing) void start()
  },
)
onBeforeUnmount(stop)
</script>
<template>
  <img
    ref="image"
    :class="{ 'media-hidden': !loaded || !playing || !native }"
    :src="src"
    :alt="alt ?? ''"
    :width="width || undefined"
    :height="height || undefined"
    :loading="external ? 'lazy' : undefined"
    :referrerpolicy="external ? 'no-referrer' : undefined"
    decoding="async"
    @load="still"
    @error="emit('error')"
  />
  <canvas
    v-show="loaded && !(playing && native)"
    ref="canvas"
    class="media-still"
    aria-hidden="true"
  />
</template>
