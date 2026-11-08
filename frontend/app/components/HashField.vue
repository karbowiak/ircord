<script setup lang="ts">
import {
  HEIGHT,
  ROCK_RADIUS,
  WIDTH,
  controlFor,
  newGame,
  noInput,
  shipPoints,
  step,
  type GameState,
  type Input,
  type Rock,
} from '../../shared/utils/hash-field'
import {
  MAX_NAME,
  placesIn,
  tableName,
  type ScoreEntry,
} from '../../shared/utils/hash-field-scores'

/**
 * The About Easter egg. Nothing animates until the player starts; the loop pauses whenever the
 * tab is hidden, the window loses focus or focus leaves the game, and everything (frame loop,
 * listeners, observer) stops when it closes. Keys stop at the game section, so the workspace
 * shortcuts (bubble-phase window listeners) never see them; type-to-compose ignores them because
 * Settings is an open dialog. Escape returns to About instead of closing Settings.
 *
 * The high score table is the server's instance-wide top 10. The High scores link above the field
 * shows it at any time (pausing a running game); the game over screen shows it too, after asking
 * for a name when the final score places.
 */
const emit = defineEmits<{ exit: [] }>()

type Phase = 'ready' | 'playing' | 'paused' | 'over'
const phase = ref<Phase>('ready')
const score = ref(0)
const lives = ref(0)
const level = ref(1)
const root = ref<HTMLElement | null>(null)
const field = ref<HTMLCanvasElement | null>(null)
const reducedMotion = ref(false)

const chat = useChatStore()
/** The table as last loaded; undefined until loaded, null when it could not be loaded. */
const table = ref<ScoreEntry[] | null | undefined>()
const tableLoading = ref(false)
/** The High scores view, over whatever the field shows. */
const showScores = ref(false)
/** The name form for a placing score; set at game over. */
const entry = ref<{ name: string; saving: boolean; error: string } | null>(null)
/** Rank of this game's entry, to highlight it; 0 when it did not place. */
const placed = ref(0)
const notice = ref('')
const nameInput = ref<HTMLInputElement | null>(null)

let game: GameState = newGame(Math.random)
let input: Input = noInput()
let frame = 0
let last = 0
let observer: ResizeObserver | undefined
let motionQuery: MediaQueryList | undefined
/** Theme colours, read when the game starts, resumes or resizes rather than on every frame. */
let palette = { ink: '#fff', accent: '#b8a4f8', muted: '#888' }
/** Short-lived fragments of broken rocks; skipped entirely under reduced motion. */
let sparks: { x: number; y: number; vx: number; vy: number; life: number }[] = []

function sync() {
  score.value = game.score
  lives.value = game.lives
  level.value = game.level
}

function start() {
  game = newGame(Math.random)
  sparks = []
  showScores.value = false
  entry.value = null
  placed.value = 0
  notice.value = ''
  sync()
  resume()
}

function resume() {
  showScores.value = false
  readColors()
  input = noInput()
  phase.value = 'playing'
  field.value?.focus({ preventScroll: true })
  last = 0
  cancelAnimationFrame(frame)
  frame = requestAnimationFrame(tick)
}

function pause() {
  if (phase.value !== 'playing') return
  phase.value = 'paused'
  stopLoop()
  draw()
}

function stopLoop() {
  cancelAnimationFrame(frame)
  frame = 0
  input = noInput()
}

function tick(now: number) {
  // Clamp long frames (a background tab, a debugger) so nothing jumps through a rock.
  const dt = last ? Math.min((now - last) / 1000, 1 / 30) : 0
  last = now
  const events = step(game, input, dt, Math.random)
  if (!reducedMotion.value) {
    for (const rock of events.broken)
      for (let i = 0; i < 6; i++) {
        const direction = Math.random() * Math.PI * 2
        const speed = 40 + Math.random() * 80
        sparks.push({
          x: rock.x,
          y: rock.y,
          vx: Math.cos(direction) * speed,
          vy: Math.sin(direction) * speed,
          life: 0.5,
        })
      }
    for (const spark of sparks) {
      spark.x += spark.vx * dt
      spark.y += spark.vy * dt
      spark.life -= dt
    }
    sparks = sparks.filter((spark) => spark.life > 0)
  }
  sync()
  draw()
  if (game.over) {
    phase.value = 'over'
    stopLoop()
    void gameOver()
    return
  }
  frame = requestAnimationFrame(tick)
}

async function loadTable() {
  tableLoading.value = true
  try {
    table.value = (await apiClient().hashFieldScores()).items
  } catch {
    table.value = null
  } finally {
    tableLoading.value = false
  }
}

function focusPrimary() {
  void nextTick(() => root.value?.querySelector<HTMLButtonElement>('[data-primary]')?.focus())
}

/** Shows the final score and the table; a placing score first asks for a name. */
async function gameOver() {
  const score = game.score
  focusPrimary()
  await loadTable()
  if (phase.value !== 'over' || game.score !== score || !table.value) return
  if (placesIn(table.value, score)) {
    const user = chat.data?.user
    entry.value = {
      name: (user?.displayName || user?.nick || '').slice(0, MAX_NAME),
      saving: false,
      error: '',
    }
    await nextTick()
    nameInput.value?.focus()
    nameInput.value?.select()
  }
}

async function saveScore() {
  const form = entry.value
  if (!form || form.saving) return
  const name = tableName(form.name)
  if (!name) {
    form.error = `Enter a name of 1 to ${MAX_NAME} characters.`
    return
  }
  form.saving = true
  form.error = ''
  try {
    const result = await apiClient().submitHashFieldScore({ name, score: game.score })
    table.value = result.items
    placed.value = result.rank
    entry.value = null
    notice.value = result.placed
      ? `Saved at #${result.rank}.`
      : 'Another score took that place first.'
    focusPrimary()
  } catch (error) {
    form.saving = false
    form.error = errorText(error)
  }
}

function skipScore() {
  entry.value = null
  focusPrimary()
}

/** The High scores link: pauses a running game and shows the table. */
function openScores() {
  if (phase.value === 'playing') pause()
  showScores.value = true
  void loadTable()
  void nextTick(() => root.value?.querySelector<HTMLButtonElement>('[data-close-scores]')?.focus())
}

function closeScores() {
  showScores.value = false
  if (phase.value === 'over' && entry.value) void nextTick(() => nameInput.value?.focus())
  else focusPrimary()
}

function readColors() {
  const style = getComputedStyle(root.value ?? document.documentElement)
  palette = {
    ink: style.getPropertyValue('--text').trim() || '#fff',
    accent: style.getPropertyValue('--accent').trim() || '#b8a4f8',
    muted: style.getPropertyValue('--muted').trim() || '#888',
  }
}

function drawHash(context: CanvasRenderingContext2D, rock: Rock) {
  const r = ROCK_RADIUS[rock.size]
  const a = r * 0.36
  context.save()
  context.translate(rock.x, rock.y)
  context.rotate(rock.angle)
  context.lineWidth = Math.max(2, r * 0.2)
  context.beginPath()
  // The IRCord mark: two slanted uprights crossed by two bars.
  context.moveTo(-a + r * 0.12, -r * 0.8)
  context.lineTo(-a - r * 0.12, r * 0.8)
  context.moveTo(a + r * 0.12, -r * 0.8)
  context.lineTo(a - r * 0.12, r * 0.8)
  context.moveTo(-r * 0.8, -a)
  context.lineTo(r * 0.8, -a)
  context.moveTo(-r * 0.8, a)
  context.lineTo(r * 0.8, a)
  context.stroke()
  context.restore()
}

function draw() {
  const canvas = field.value
  const context = canvas?.getContext('2d')
  if (!canvas || !context) return
  const { ink, accent, muted } = palette
  const scale = canvas.width / WIDTH
  context.setTransform(scale, 0, 0, scale, 0, 0)
  context.clearRect(0, 0, WIDTH, HEIGHT)
  context.lineCap = 'round'
  context.lineJoin = 'round'

  context.strokeStyle = muted
  for (const rock of game.rocks) drawHash(context, rock)

  context.fillStyle = ink
  for (const bullet of game.bullets) {
    context.beginPath()
    context.arc(bullet.x, bullet.y, 2, 0, Math.PI * 2)
    context.fill()
  }
  context.fillStyle = accent
  for (const spark of sparks) {
    context.globalAlpha = Math.max(0, spark.life * 2)
    context.fillRect(spark.x - 1, spark.y - 1, 2, 2)
  }
  context.globalAlpha = 1

  if (!game.over) {
    const ship = game.ship
    const points = shipPoints(ship)
    context.strokeStyle = accent
    context.lineWidth = 2
    // A shielded (just spawned) ship is drawn faint rather than blinking.
    context.globalAlpha = ship.shield > 0 ? 0.45 : 1
    context.beginPath()
    context.moveTo(points[0]!.x, points[0]!.y)
    for (const point of points.slice(1)) context.lineTo(point.x, point.y)
    context.closePath()
    context.stroke()
    if (input.thrust && phase.value === 'playing') {
      const tail = points[2]!
      const back = { x: tail.x - Math.cos(ship.angle) * 7, y: tail.y - Math.sin(ship.angle) * 7 }
      context.beginPath()
      context.moveTo(tail.x, tail.y)
      context.lineTo(back.x, back.y)
      context.stroke()
    }
    context.globalAlpha = 1
  }
}

function resize() {
  const canvas = field.value
  if (!canvas) return
  readColors()
  const width = Math.max(1, Math.round(canvas.clientWidth * (window.devicePixelRatio || 1)))
  if (canvas.width !== width) {
    canvas.width = width
    canvas.height = Math.round((width * HEIGHT) / WIDTH)
  }
  draw()
}

/** Every key pressed in the game stays in the game. */
function keydown(event: KeyboardEvent) {
  event.stopPropagation()
  if (event.key === 'Escape') {
    // Close the table, or return to About; keep Settings open.
    event.preventDefault()
    if (showScores.value) return closeScores()
    return emit('exit')
  }
  if (event.metaKey || event.ctrlKey || event.altKey) return
  if ((event.key === 'p' || event.key === 'P') && !(event.target instanceof HTMLInputElement)) {
    event.preventDefault()
    if (phase.value === 'playing') pause()
    else if (phase.value === 'paused') resume()
    return
  }
  // Steering only applies to the playfield itself; focused buttons keep Space and Enter.
  if (event.target !== field.value) return
  const control = controlFor(event.key)
  if (control) {
    event.preventDefault()
    if (phase.value === 'playing') input[control] = true
    else if (control === 'fire' && !event.repeat) phase.value === 'paused' ? resume() : start()
  } else if (event.key === 'Enter' && phase.value !== 'playing') {
    event.preventDefault()
    if (phase.value === 'paused') resume()
    else start()
  }
}

function keyup(event: KeyboardEvent) {
  event.stopPropagation()
  const control = controlFor(event.key)
  if (control) input[control] = false
}

/** Touch controls are held, not clicked. */
function hold(control: keyof Input, down: boolean) {
  if (phase.value === 'playing') input[control] = down
}

function focusout(event: FocusEvent) {
  const next = event.relatedTarget
  if (!(next instanceof Node) || !root.value?.contains(next)) pause()
}
function visibility() {
  if (document.visibilityState === 'hidden') pause()
}
function motionChanged() {
  reducedMotion.value = !!motionQuery?.matches
  if (reducedMotion.value) sparks = []
}

onMounted(() => {
  motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
  motionChanged()
  motionQuery.addEventListener('change', motionChanged)
  document.addEventListener('visibilitychange', visibility)
  window.addEventListener('blur', pause)
  observer = new ResizeObserver(resize)
  if (field.value) observer.observe(field.value)
  sync()
  resize()
  root.value?.querySelector<HTMLButtonElement>('[data-primary]')?.focus()
})
onBeforeUnmount(() => {
  stopLoop()
  observer?.disconnect()
  motionQuery?.removeEventListener('change', motionChanged)
  document.removeEventListener('visibilitychange', visibility)
  window.removeEventListener('blur', pause)
})
</script>
<template>
  <section
    ref="root"
    class="hash-field"
    aria-labelledby="hash-field-title"
    :data-phase="phase"
    @keydown="keydown"
    @keyup="keyup"
    @focusout="focusout"
  >
    <header class="hash-field-header">
      <h4 id="hash-field-title">Hash Field</h4>
      <button
        type="button"
        class="text-button hash-field-scores-link"
        :aria-pressed="showScores"
        @pointerdown.prevent
        @click="showScores ? closeScores() : openScores()"
      >
        High scores
      </button>
      <p
        class="hash-field-score"
        aria-live="off"
      >
        <span>Score {{ score }}</span
        ><span>Lives {{ lives }}</span
        ><span>Level {{ level }}</span>
      </p>
    </header>
    <div
      class="hash-field-stage"
      :class="{ board: showScores || phase === 'over' }"
    >
      <canvas
        ref="field"
        tabindex="0"
        role="img"
        aria-label="Hash Field playfield"
        aria-describedby="hash-field-help"
      />
      <div
        v-if="showScores"
        class="hash-field-overlay"
      >
        <HashFieldTable
          :items="table"
          :loading="tableLoading"
          :highlight="phase === 'over' ? placed : 0"
        />
        <button
          class="button primary small"
          data-close-scores
          @click="closeScores()"
        >
          {{ phase === 'playing' || phase === 'paused' ? 'Back to the game' : 'Back' }}
        </button>
      </div>
      <div
        v-else-if="phase === 'over'"
        class="hash-field-overlay"
        aria-labelledby="hash-field-over-title"
        role="group"
      >
        <h5
          id="hash-field-over-title"
          class="hash-field-over-title"
        >
          Game over
        </h5>
        <p role="status">
          Final score <strong class="hash-field-final">{{ score.toLocaleString('en-US') }}</strong>
          <template v-if="notice"> · {{ notice }}</template>
        </p>
        <form
          v-if="entry"
          class="hash-field-entry"
          @submit.prevent="saveScore()"
        >
          <p>That score makes the top 10. Enter your name for the table.</p>
          <div class="hash-field-entry-row">
            <input
              ref="nameInput"
              v-model="entry.name"
              class="text-input"
              aria-label="Name for the high score table"
              :maxlength="MAX_NAME"
              autocomplete="nickname"
              spellcheck="false"
              :disabled="entry.saving"
            />
            <button
              type="submit"
              class="button primary small"
              :disabled="entry.saving"
            >
              Save score
            </button>
            <button
              type="button"
              class="button secondary small"
              :disabled="entry.saving"
              @click="skipScore()"
            >
              Skip
            </button>
          </div>
          <p
            v-if="entry.error"
            class="hash-field-error"
            role="alert"
          >
            {{ entry.error }}
          </p>
        </form>
        <HashFieldTable
          :items="table"
          :loading="tableLoading"
          :highlight="placed"
        />
        <button
          class="button primary small"
          data-primary
          @click="start()"
        >
          Play again
        </button>
      </div>
      <div
        v-else-if="phase !== 'playing'"
        class="hash-field-overlay"
        role="status"
      >
        <template v-if="phase === 'ready'">
          <p>Break up the drifting hash marks. Bigger ones split in two.</p>
        </template>
        <template v-else><p>Paused</p></template>
        <button
          class="button primary small"
          data-primary
          @click="phase === 'paused' ? resume() : start()"
        >
          {{ phase === 'paused' ? 'Resume' : 'Start' }}
        </button>
      </div>
    </div>
    <div
      class="hash-field-touch"
      aria-hidden="true"
    >
      <button
        v-for="control in [
          { id: 'left', icon: '◀' },
          { id: 'thrust', icon: '▲' },
          { id: 'right', icon: '▶' },
          { id: 'fire', icon: '●' },
        ] as const"
        :key="control.id"
        tabindex="-1"
        :data-control="control.id"
        @pointerdown.prevent="hold(control.id, true)"
        @pointerup="hold(control.id, false)"
        @pointercancel="hold(control.id, false)"
        @pointerleave="hold(control.id, false)"
        @contextmenu.prevent
      >
        {{ control.icon }}
      </button>
    </div>
    <p
      id="hash-field-help"
      class="settings-description"
    >
      ← → or A D steer · ↑ or W thrust · Space fires · P pauses · Esc returns to About.
    </p>
    <!-- Pointer presses keep focus on the playfield: where a click does not focus a button (Safari,
         macOS Firefox) the blur would pause first and the click would then toggle straight back. -->
    <div class="hash-field-actions">
      <button
        v-if="phase === 'playing' || phase === 'paused'"
        class="button secondary small"
        @pointerdown.prevent
        @click="phase === 'playing' ? pause() : resume()"
      >
        {{ phase === 'playing' ? 'Pause' : 'Resume' }}
      </button>
      <button
        v-if="phase !== 'ready'"
        class="button secondary small"
        @pointerdown.prevent
        @click="start()"
      >
        Restart
      </button>
      <button
        class="button secondary small"
        @click="emit('exit')"
      >
        Back to About
      </button>
    </div>
  </section>
</template>
