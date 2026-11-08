/**
 * The Hash Field high score table: the instance-wide top 10, kept by the server (Go in
 * production, the demo server otherwise). These are the rules both servers and the game share.
 */

export const TABLE_SIZE = 10
export const MAX_NAME = 24
export const MAX_SCORE = 10_000_000

export interface ScoreEntry {
  rank: number
  name: string
  score: number
}

/** The entries a new table starts with. */
export const SEED_SCORES: readonly Omit<ScoreEntry, 'rank'>[] = [
  { name: 'Michael Karbowiak', score: 97909 },
  { name: 'Peter Petermann', score: 22831 },
]

/** Whether a finished game's score would make the table: ties keep the earlier entry. */
export function placesIn(table: readonly ScoreEntry[], score: number): boolean {
  if (!validScore(score)) return false
  return table.length < TABLE_SIZE || score > table[table.length - 1]!.score
}

/** Every Hash Field score is a positive multiple of 10 below the sane maximum. */
export function validScore(score: number): boolean {
  return Number.isSafeInteger(score) && score > 0 && score <= MAX_SCORE && score % 10 === 0
}

/** Why a name was refused, worded as the Go server words it. */
export const NAME_LENGTH = `Enter a name of 1 to ${MAX_NAME} characters.`
export const NAME_CHARACTERS = 'That name has characters the table can’t show.'

/**
 * Letters, marks, numbers, punctuation, symbols and spaces, plus the zero-width joiner inside
 * emoji sequences. Controls, other format characters (bidi overrides), private use, unassigned
 * code points, lone surrogates and U+FFFD are refused, as the Go server does.
 */
const ALLOWED = /^[\p{L}\p{M}\p{N}\p{P}\p{S}\p{Zs}\s\u200d]*$/u

/** A table name: trimmed, whitespace runs folded, 1 to 24 characters (code points). */
export function tableName(raw: string): { name: string } | { error: string } {
  if (!ALLOWED.test(raw) || raw.includes('\ufffd')) return { error: NAME_CHARACTERS }
  const name = raw.trim().split(/\s+/).join(' ')
  if (!name || [...name].length > MAX_NAME) return { error: NAME_LENGTH }
  return { name }
}

/** The first `MAX_NAME` characters of a name, never splitting an astral character. */
export const clipName = (name: string) => [...name].slice(0, MAX_NAME).join('')

/** Inserts a score after any equal ones and keeps the top entries. Returns the new rank or 0. */
export function insertScore(
  table: readonly Omit<ScoreEntry, 'rank'>[],
  name: string,
  score: number,
): { rank: number; items: ScoreEntry[] } {
  const rows = [...table]
  let at = rows.findIndex((row) => row.score < score)
  if (at < 0) at = rows.length
  rows.splice(at, 0, { name, score })
  const items = rows.slice(0, TABLE_SIZE).map((row, i) => ({ ...row, rank: i + 1 }))
  return { rank: at < TABLE_SIZE ? at + 1 : 0, items }
}
