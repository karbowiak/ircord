import { test } from 'node:test'
import assert from 'node:assert/strict'
import { aboutBuild } from '../shared/utils/about'
import {
  HEIGHT,
  LIVES,
  ROCK_POINTS,
  WIDTH,
  controlFor,
  distance,
  newGame,
  noInput,
  spawnRocks,
  step,
  type GameState,
  type Rock,
} from '../shared/utils/hash-field'

/** A small deterministic generator so every run plays the same game. */
function seeded(seed = 1) {
  let value = seed
  return () => {
    value = (value * 16807) % 2147483647
    return (value - 1) / 2147483646
  }
}

const still = (x: number, y: number, size: Rock['size']): Rock => ({
  x,
  y,
  vx: 0,
  vy: 0,
  size,
  angle: 0,
  spin: 0,
})

test('About shows release tags and short build ids, and nothing else', () => {
  assert.deepEqual(aboutBuild('v0.1.2', '8b1c3f2e-1111-2222-3333-444455556666'), {
    release: 'v0.1.2',
    id: '8b1c3f2e',
  })
  assert.deepEqual(aboutBuild('v1.0.0-rc.1', 'dev'), { release: 'v1.0.0-rc.1', id: 'dev' })
  // Development and untagged builds have no release; a stray value is never echoed.
  assert.deepEqual(aboutBuild('', ''), { release: '', id: '' })
  assert.deepEqual(aboutBuild(undefined, undefined), { release: '', id: '' })
  assert.equal(aboutBuild('main', 'x').release, '')
  assert.equal(aboutBuild('v1.2', 'x').release, '')
  assert.equal(aboutBuild('<b>v1</b>', '<script>').id, '')
})

test('Hash Field starts with full lives and rocks away from the ship', () => {
  const game = newGame(seeded())
  assert.equal(game.lives, LIVES)
  assert.equal(game.score, 0)
  assert.equal(game.rocks.length, 4)
  for (const rock of game.rocks) {
    assert.equal(rock.size, 3)
    assert.ok(distance(rock, game.ship) > 100)
  }
  assert.equal(spawnRocks(20, seeded()).length, 9)
})

test('Hash Field wraps the ship and measures distance across the edges', () => {
  const game = newGame(seeded())
  game.rocks = [still(100, 100, 3)]
  game.ship.x = WIDTH - 1
  game.ship.vx = 200
  step(game, noInput(), 0.02, seeded())
  assert.ok(game.ship.x < 10)
  assert.equal(distance({ x: 1, y: 1 }, { x: WIDTH - 1, y: HEIGHT - 1 }), Math.hypot(2, 2))
})

test('a bullet splits a big hash into two smaller ones and scores it', () => {
  const random = seeded()
  const game: GameState = newGame(random)
  game.rocks = [still(game.ship.x, game.ship.y - 60, 3)]
  game.ship.shield = 0
  const fire = { ...noInput(), fire: true }
  step(game, fire, 0.01, random)
  assert.equal(game.bullets.length, 1)
  for (let i = 0; i < 30 && game.score === 0; i++) step(game, noInput(), 0.01, random)
  assert.equal(game.score, ROCK_POINTS[3])
  assert.deepEqual(
    game.rocks.map((rock) => rock.size),
    [2, 2],
  )
  assert.equal(game.bullets.length, 0)
})

test('the smallest hash breaks without splitting, and clearing a level starts the next', () => {
  const random = seeded()
  const game = newGame(random)
  game.rocks = [still(game.ship.x, game.ship.y - 40, 1)]
  game.ship.shield = 0
  step(game, { ...noInput(), fire: true }, 0.01, random)
  for (let i = 0; i < 30 && game.level === 1; i++) step(game, noInput(), 0.01, random)
  assert.equal(game.score, ROCK_POINTS[1])
  assert.equal(game.level, 2)
  assert.equal(game.rocks.length, 5)
  assert.ok(game.ship.shield > 0)
})

test('firing is rate limited', () => {
  const random = seeded()
  const game = newGame(random)
  game.rocks = [still(10, 10, 3)]
  const fire = { ...noInput(), fire: true }
  for (let i = 0; i < 10; i++) step(game, fire, 0.01, random)
  assert.equal(game.bullets.length, 1)
})

test('a shielded ship survives a hit; an unshielded one loses a life, the last one ends it', () => {
  const random = seeded()
  const game = newGame(random)
  game.rocks = [still(game.ship.x, game.ship.y, 3), still(10, 10, 3)]
  step(game, noInput(), 0.01, random)
  assert.equal(game.lives, LIVES)

  game.ship.shield = 0
  const events = step(game, noInput(), 0.01, random)
  assert.equal(events.shipLost, true)
  assert.equal(game.lives, LIVES - 1)
  assert.ok(game.ship.shield > 0, 'the new ship starts shielded')

  game.lives = 1
  game.ship.shield = 0
  game.rocks.push(still(game.ship.x, game.ship.y, 2))
  step(game, noInput(), 0.01, random)
  assert.equal(game.over, true)
  const frozen = JSON.stringify(game)
  step(game, { ...noInput(), thrust: true, fire: true }, 0.5, random)
  assert.equal(JSON.stringify(game), frozen, 'a finished game no longer moves')
})

test('Hash Field keys: arrows and WASD steer, Space fires, nothing else is claimed', () => {
  assert.equal(controlFor('ArrowLeft'), 'left')
  assert.equal(controlFor('d'), 'right')
  assert.equal(controlFor('W'), 'thrust')
  assert.equal(controlFor(' '), 'fire')
  assert.equal(controlFor('Enter'), undefined)
  assert.equal(controlFor('x'), undefined)
})
