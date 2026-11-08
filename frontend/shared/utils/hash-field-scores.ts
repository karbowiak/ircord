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

/**
 * A table name: trimmed, whitespace runs folded, 1 to 24 characters. Control and format
 * characters (bidi overrides, zero-width joiners) are refused, as the Go server does.
 */
export function tableName(raw: string): string | undefined {
  // eslint-disable-next-line no-control-regex
  if (/[\u0000-\u0008\u000e-\u001f\u007f-\u0084\u0086-\u009f\p{Cf}�]/u.test(raw)) return undefined
  const name = raw.trim().split(/\s+/).join(' ')
  if (!name || [...name].length > MAX_NAME) return undefined
  return name
}

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
