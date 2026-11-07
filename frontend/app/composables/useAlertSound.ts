const key = 'ircord.device.sound'
export type SoundMode = 'off' | 'mentions' | 'all'
const mode = ref<SoundMode>(readLocal(key, (v) => (v === 'off' || v === 'all' ? v : 'mentions')))
let context: AudioContext | undefined
let lastPlayed = 0

/**
 * A short two-note chime made with WebAudio (no sound files). Sounds are a per-device choice;
 * browsers only allow them after the user has interacted with the page.
 */
export function useAlertSound() {
  function play(kind: 'mention' | 'message') {
    if (mode.value === 'off' || (mode.value === 'mentions' && kind !== 'mention')) return
    // A burst of messages chimes once.
    if (Date.now() - lastPlayed < 1500) return
    lastPlayed = Date.now()
    try {
      context ??= new AudioContext()
      if (context.state === 'suspended') void context.resume()
      const start = context.currentTime
      const notes = kind === 'mention' ? [880, 1320] : [660]
      notes.forEach((frequency, i) => {
        const oscillator = context!.createOscillator()
        const gain = context!.createGain()
        oscillator.type = 'sine'
        oscillator.frequency.value = frequency
        const at = start + i * 0.12
        gain.gain.setValueAtTime(0.0001, at)
        gain.gain.exponentialRampToValueAtTime(0.18, at + 0.01)
        gain.gain.exponentialRampToValueAtTime(0.0001, at + 0.25)
        oscillator.connect(gain).connect(context!.destination)
        oscillator.start(at)
        oscillator.stop(at + 0.3)
      })
    } catch {
      /* No audio on this device. */
    }
  }
  function setMode(value: SoundMode) {
    mode.value = value
    writeLocal(key, value)
  }
  return { mode, setMode, play, preview: () => play('mention') }
}
