import assert from 'node:assert/strict'
import { effectScope, nextTick, ref, shallowRef } from 'vue'
import { useInlinePlayback } from '../app/composables/useInlinePlayback.ts'

const flush = async () => {
  for (let i = 0; i < 8; i++) await Promise.resolve()
  await nextTick()
}
const active = ref(true)
const preview = ref(false)
const errors = []
let playback
const media = {
  paused: true,
  muted: true,
  plays: 0,
  pauses: 0,
  play() {
    this.plays++
    this.paused = false
    queueMicrotask(() => playback.played())
    return Promise.resolve()
  },
  pause() {
    if (this.paused) return
    this.pauses++
    this.paused = true
    queueMicrotask(() => playback.paused())
  },
}
const player = shallowRef(media)
const scope = effectScope()
playback = scope.run(() =>
  useInlinePlayback(player, active, preview, (error) => errors.push(error)),
)
try {
  preview.value = true
  await flush()
  assert.equal(media.plays, 1)
  assert(playback.playing.value)

  media.muted = false
  playback.volumeChanged()
  for (let i = 0; i < 4; i++) {
    preview.value = false
    await flush()
    preview.value = true
    await flush()
  }
  assert.equal(media.muted, false, 'native unmute must never be overwritten by hover sync')
  assert.equal(
    media.pauses,
    0,
    'native control pointer/focus changes must not pause audible playback',
  )
  assert.equal(media.plays, 1, 'observed play events must not feed new play commands')

  playback.takeControl()
  media.pause()
  await flush()
  preview.value = false
  await flush()
  preview.value = true
  await flush()
  assert(media.paused, 'explicit pause survives hover/focus changes')
  assert.equal(playback.playing.value, false)
  media.play()
  await flush()
  active.value = false
  await flush()
  assert(media.paused, 'hidden, off-screen or expanded-viewer media must stop')
  active.value = true
  await flush()
  assert(media.paused, 'manual playback does not restart when returning to the page')

  preview.value = false
  playback.reset()
  media.muted = true
  await flush()
  preview.value = true
  await flush()
  assert(!media.paused, 'a fresh muted preview can still follow hover')
  preview.value = false
  await flush()
  assert(media.paused, 'passive previews stop when hover/focus ends')

  const originalPlay = media.play
  media.play = () => Promise.reject(new DOMException('Blocked fixture', 'NotAllowedError'))
  preview.value = true
  await flush()
  assert.equal(errors.length, 0, 'a blocked automatic preview keeps native controls available')
  media.play = () => Promise.reject(new DOMException('Unsupported fixture', 'NotSupportedError'))
  playback.sync()
  await flush()
  assert.equal(errors.length, 1, 'actual media failures still reach the normal fallback')
  media.play = originalPlay
  playback.takeControl()
  media.play()
  await flush()
} finally {
  scope.stop()
}
assert(media.paused, 'disposing the media controller stops playback')
console.log(
  'Muted hover, native unmute/manual controls, focus changes, suspension, disposal and real playback errors pass.',
)
