/**
 * Plays streamed float32 mono PCM without gaps: each chunk becomes an AudioBuffer scheduled to
 * start where the previous one ends. Playback starts once `prebuffer` seconds are queued, so the
 * engine's tiny first chunks do not stutter; `buffered` waits for the whole line instead.
 */
export class PcmPlayer {
  private queued: AudioBuffer[] = []
  private sources = new Set<AudioBufferSourceNode>()
  private started = false
  private stopped = false
  private nextTime = 0
  private queuedSeconds = 0
  /** Seconds of audio received. */
  seconds = 0
  private finished?: () => void

  constructor(
    private readonly context: AudioContext,
    private readonly sampleRate: number,
    private readonly options: { prebuffer?: number; buffered?: boolean } = {},
  ) {}

  push(pcm: Float32Array) {
    if (this.stopped || !pcm.length) return
    const buffer = this.context.createBuffer(1, pcm.length, this.sampleRate)
    buffer.copyToChannel(pcm as Float32Array<ArrayBuffer>, 0)
    this.seconds += buffer.duration
    if (this.started) return this.schedule(buffer)
    this.queued.push(buffer)
    this.queuedSeconds += buffer.duration
    if (!this.options.buffered && this.queuedSeconds >= (this.options.prebuffer ?? 0.3))
      this.start()
  }

  private start() {
    this.started = true
    this.nextTime = this.context.currentTime + 0.05
    for (const buffer of this.queued) this.schedule(buffer)
    this.queued = []
  }

  private schedule(buffer: AudioBuffer) {
    const source = this.context.createBufferSource()
    source.buffer = buffer
    source.connect(this.context.destination)
    // Generation fell behind: resume right away instead of in the past.
    const at = Math.max(this.nextTime, this.context.currentTime + 0.02)
    source.start(at)
    this.nextTime = at + buffer.duration
    this.sources.add(source)
    source.onended = () => {
      this.sources.delete(source)
      if (!this.sources.size && this.finished) this.finished()
    }
  }

  /** All audio is in: plays what is queued and resolves when the last sound has ended. */
  end() {
    if (!this.started) this.start()
    return new Promise<void>((resolve) => {
      if (this.stopped || !this.sources.size) return resolve()
      this.finished = resolve
    })
  }

  stop() {
    this.stopped = true
    this.queued = []
    for (const source of this.sources) {
      source.onended = null
      try {
        source.stop()
      } catch {
        /* Not started yet. */
      }
    }
    this.sources.clear()
    this.finished?.()
  }
}
