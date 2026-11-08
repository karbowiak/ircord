import {
  SEED_SCORES,
  insertScore,
  tableName,
  validScore,
  type ScoreEntry,
} from '../../shared/utils/hash-field-scores'

/*
 * Mock of the Go service's Hash Field high scores: one table for the whole demo instance, kept in
 * the data storage, with the same validation and per-account submission limits.
 */
const storage = () => useStorage('data')
const KEY = 'hash-field:scores'
const GAP = 10_000
const HOURLY = 20
const submissions = new Map<string, number[]>()

type Row = Omit<ScoreEntry, 'rank'>
async function rows(): Promise<Row[]> {
  return (await storage().getItem<Row[]>(KEY)) ?? [...SEED_SCORES]
}
const ranked = (table: Row[]): ScoreEntry[] => table.map((row, i) => ({ ...row, rank: i + 1 }))

export async function hashFieldTable() {
  return { items: ranked(await rows()) }
}

let pending = Promise.resolve()
export async function submitHashFieldScore(account: string, body: unknown) {
  const input = (body ?? {}) as { name?: unknown; score?: unknown }
  const checked =
    typeof input.name === 'string' ? tableName(input.name) : { error: 'Enter a name.' }
  if ('error' in checked) throw createError({ statusCode: 422, statusMessage: checked.error })
  const name = checked.name
  if (typeof input.score !== 'number' || !validScore(input.score))
    throw createError({ statusCode: 422, statusMessage: 'that is not a Hash Field score' })
  const now = Date.now()
  const recent = (submissions.get(account) ?? []).filter((at) => at > now - 3_600_000)
  if ((recent.length && now - recent[recent.length - 1]! < GAP) || recent.length >= HOURLY) {
    submissions.set(account, recent)
    throw createError({ statusCode: 429, statusMessage: 'too many scores; try again shortly' })
  }
  submissions.set(account, [...recent, now])
  const score = input.score
  // Serialise read-modify-write so two submissions cannot overwrite each other.
  const run = pending.then(async () => {
    const result = insertScore(await rows(), name, score)
    if (result.rank)
      await storage().setItem(
        KEY,
        result.items.map(({ name, score }) => ({ name, score })),
      )
    return { placed: result.rank > 0, rank: result.rank, items: result.items }
  })
  pending = run.then(
    () => undefined,
    () => undefined,
  )
  return run
}
