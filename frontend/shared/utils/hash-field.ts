/**
 * Hash Field, the Settings → About Easter egg: steer a small ship through a field of drifting hash
 * marks and break them apart. This module is the whole game as plain data and pure steps, so the
 * rules are tested without a canvas; HashField.vue only draws the state and feeds it input.
 */

export const WIDTH = 480
export const HEIGHT = 360

/** Rock sizes: a size-3 hash splits into two size-2 hashes, then two size-1, then breaks. */
export const ROCK_RADIUS = [0, 9, 17, 28] as const
export const ROCK_POINTS = [0, 100, 50, 20] as const

const TURN = 4.2 // radians per second
const THRUST = 220 // px/s²
const DRAG = 0.55 // fraction of speed kept per second
const MAX_SPEED = 260
const SHIP_RADIUS = 9
const BULLET_SPEED = 380
const BULLET_LIFE = 0.9 // seconds
const FIRE_GAP = 0.22 // seconds between shots
const MAX_BULLETS = 4
const RESPAWN_SHIELD = 2.5 // seconds a new ship cannot be hit
export const LIVES = 3

export interface Vec {
  x: number
  y: number
}
export interface Ship extends Vec {
  vx: number
  vy: number
  angle: number
  /** Seconds left before the ship can be hit. */
  shield: number
}
export interface Rock extends Vec {
  vx: number
  vy: number
  size: 1 | 2 | 3
  angle: number
  spin: number
}
export interface Bullet extends Vec {
  vx: number
  vy: number
  life: number
}
export interface GameState {
  ship: Ship
  rocks: Rock[]
  bullets: Bullet[]
  score: number
  lives: number
  level: number
  /** Seconds until the next shot may fire. */
  cooldown: number
  over: boolean
}
export interface Input {
  left: boolean
  right: boolean
  thrust: boolean
  fire: boolean
}
/** A source of numbers in [0, 1); Math.random in play, a seeded one in tests. */
export type Random = () => number

export const noInput = (): Input => ({ left: false, right: false, thrust: false, fire: false })

function centeredShip(): Ship {
  return { x: WIDTH / 2, y: HEIGHT / 2, vx: 0, vy: 0, angle: -Math.PI / 2, shield: RESPAWN_SHIELD }
}

/** Places the rocks for a level away from the ship, each drifting in its own direction. */
export function spawnRocks(level: number, random: Random): Rock[] {
  const rocks: Rock[] = []
  const count = Math.min(3 + level, 9)
  for (let i = 0; i < count; i++) {
    // Start on the field's edge so nothing appears on top of the ship in the centre.
    const edge = random() * 2 * (WIDTH + HEIGHT)
    const x =
      edge < WIDTH
        ? edge
        : edge < WIDTH + HEIGHT
          ? WIDTH
          : edge < 2 * WIDTH + HEIGHT
            ? edge - WIDTH - HEIGHT
            : 0
    const y =
      edge < WIDTH
        ? 0
        : edge < WIDTH + HEIGHT
          ? edge - WIDTH
          : edge < 2 * WIDTH + HEIGHT
            ? HEIGHT
            : edge - 2 * WIDTH - HEIGHT
    rocks.push(rock(x, y, 3, level, random))
  }
  return rocks
}

function rock(x: number, y: number, size: Rock['size'], level: number, random: Random): Rock {
  const direction = random() * Math.PI * 2
  const speed = (18 + random() * 30) * (1 + (3 - size) * 0.45) * (1 + (level - 1) * 0.08)
  return {
    x,
    y,
    vx: Math.cos(direction) * speed,
    vy: Math.sin(direction) * speed,
    size,
    angle: random() * Math.PI,
    spin: (random() - 0.5) * 1.6,
  }
}

export function newGame(random: Random): GameState {
  return {
    ship: centeredShip(),
    rocks: spawnRocks(1, random),
    bullets: [],
    score: 0,
    lives: LIVES,
    level: 1,
    cooldown: 0,
    over: false,
  }
}

const wrap = (value: number, size: number) => ((value % size) + size) % size

/** Distance on the wrapping field: something near one edge is also near the opposite one. */
export function distance(a: Vec, b: Vec): number {
  const dx = Math.abs(a.x - b.x)
  const dy = Math.abs(a.y - b.y)
  return Math.hypot(Math.min(dx, WIDTH - dx), Math.min(dy, HEIGHT - dy))
}

/** What a step changed, for effects the view may draw (and skips under reduced motion). */
export interface StepEvents {
  broken: Rock[]
  shipLost: boolean
  levelCleared: boolean
}

/**
 * Advances the game by `dt` seconds (the caller clamps it, so a long frame cannot tunnel through
 * rocks). Mutates and returns `state`.
 */
export function step(state: GameState, input: Input, dt: number, random: Random): StepEvents {
  const events: StepEvents = { broken: [], shipLost: false, levelCleared: false }
  if (state.over) return events
  const { ship } = state

  if (input.left) ship.angle -= TURN * dt
  if (input.right) ship.angle += TURN * dt
  if (input.thrust) {
    ship.vx += Math.cos(ship.angle) * THRUST * dt
    ship.vy += Math.sin(ship.angle) * THRUST * dt
  }
  const keep = Math.pow(DRAG, dt)
  ship.vx *= keep
  ship.vy *= keep
  const speed = Math.hypot(ship.vx, ship.vy)
  if (speed > MAX_SPEED) {
    ship.vx *= MAX_SPEED / speed
    ship.vy *= MAX_SPEED / speed
  }
  ship.x = wrap(ship.x + ship.vx * dt, WIDTH)
  ship.y = wrap(ship.y + ship.vy * dt, HEIGHT)
  ship.shield = Math.max(0, ship.shield - dt)

  state.cooldown = Math.max(0, state.cooldown - dt)
  if (input.fire && state.cooldown === 0 && state.bullets.length < MAX_BULLETS) {
    state.bullets.push({
      x: ship.x + Math.cos(ship.angle) * SHIP_RADIUS,
      y: ship.y + Math.sin(ship.angle) * SHIP_RADIUS,
      vx: ship.vx + Math.cos(ship.angle) * BULLET_SPEED,
      vy: ship.vy + Math.sin(ship.angle) * BULLET_SPEED,
      life: BULLET_LIFE,
    })
    state.cooldown = FIRE_GAP
  }

  for (const bullet of state.bullets) {
    bullet.x = wrap(bullet.x + bullet.vx * dt, WIDTH)
    bullet.y = wrap(bullet.y + bullet.vy * dt, HEIGHT)
    bullet.life -= dt
  }
  for (const r of state.rocks) {
    r.x = wrap(r.x + r.vx * dt, WIDTH)
    r.y = wrap(r.y + r.vy * dt, HEIGHT)
    r.angle += r.spin * dt
  }

  // Bullets break rocks; each bullet breaks at most one.
  const bullets: Bullet[] = []
  for (const bullet of state.bullets) {
    if (bullet.life <= 0) continue
    const hit = state.rocks.find((r) => distance(bullet, r) < ROCK_RADIUS[r.size])
    if (!hit) {
      bullets.push(bullet)
      continue
    }
    breakRock(state, hit, random)
    events.broken.push(hit)
  }
  state.bullets = bullets

  if (ship.shield === 0) {
    const hit = state.rocks.find((r) => distance(ship, r) < ROCK_RADIUS[r.size] + SHIP_RADIUS * 0.7)
    if (hit) {
      breakRock(state, hit, random)
      events.broken.push(hit)
      events.shipLost = true
      state.lives -= 1
      if (state.lives <= 0) state.over = true
      else state.ship = centeredShip()
    }
  }

  if (!state.over && state.rocks.length === 0) {
    state.level += 1
    state.rocks = spawnRocks(state.level, random)
    state.ship.shield = RESPAWN_SHIELD
    events.levelCleared = true
  }
  return events
}

function breakRock(state: GameState, hit: Rock, random: Random) {
  state.score += ROCK_POINTS[hit.size]
  state.rocks = state.rocks.filter((r) => r !== hit)
  if (hit.size === 1) return
  const size = (hit.size - 1) as Rock['size']
  state.rocks.push(
    rock(hit.x, hit.y, size, state.level, random),
    rock(hit.x, hit.y, size, state.level, random),
  )
}

/** The ship's outline: nose, right wing, notch, left wing. */
export function shipPoints(ship: Ship): Vec[] {
  const at = (angle: number, length: number) => ({
    x: ship.x + Math.cos(ship.angle + angle) * length,
    y: ship.y + Math.sin(ship.angle + angle) * length,
  })
  return [
    at(0, SHIP_RADIUS * 1.5),
    at(2.45, SHIP_RADIUS),
    at(Math.PI, SHIP_RADIUS * 0.45),
    at(-2.45, SHIP_RADIUS),
  ]
}

/** Maps a keyboard key to the control it holds, if any. */
export function controlFor(key: string): keyof Input | undefined {
  switch (key) {
    case 'ArrowLeft':
    case 'a':
    case 'A':
      return 'left'
    case 'ArrowRight':
    case 'd':
    case 'D':
      return 'right'
    case 'ArrowUp':
    case 'w':
    case 'W':
      return 'thrust'
    case ' ':
      return 'fire'
  }
  return undefined
}
