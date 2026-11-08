import { createHash, randomBytes, randomUUID, scryptSync, timingSafeEqual } from 'node:crypto'
import { deleteCookie, getCookie, setCookie, type H3Event } from 'h3'
import type { Account } from '../../shared/types/chat'
import { nicknameError } from '../../shared/utils/nickname'

/*
 * Mock of the Go service's accounts (#64): a username and password, where the username is also
 * the account and nick on the associated IRC server, so names the demo network already uses
 * are refused. Signing in exchanges them for an opaque session cookie. Messages match the Go
 * service's. Accounts unused for 90 days are deleted.
 */
const DAY = 86_400_000
const RETENTION = 90 * DAY
const storage = () => useStorage('data')
const sha256 = (value: string) => createHash('sha256').update(value).digest('hex')
const fold = (name: string) => name.toLowerCase()

interface StoredAccount extends Omit<Account, 'expiresAt'> {
  /** scrypt salt and hash, hex, `salt:hash`. */
  passwordHash: string
}
const view = (account: StoredAccount): Account => ({
  id: account.id,
  username: account.username,
  createdAt: account.createdAt,
  lastSeenAt: account.lastSeenAt,
  expiresAt: new Date(new Date(account.lastSeenAt).getTime() + RETENTION).toISOString(),
})
const fail = (statusCode: number, statusMessage: string) =>
  createError({ statusCode, statusMessage })

function startSession(event: H3Event, accountId: string) {
  const token = randomBytes(32).toString('base64url')
  void storage().setItem(`auth:sessions:${sha256(token)}`, { accountId, createdAt: Date.now() })
  setCookie(event, 'ircord_session', token, {
    httpOnly: true,
    sameSite: 'lax',
    path: '/',
    maxAge: RETENTION / 1000,
    secure: getRequestURL(event).protocol === 'https:',
  })
}

// ---- Usernames and passwords ----------------------------------------------------------------

/** Nicks the demo network already has (its members and services), and reserved names. */
const networkNames = new Set([
  ...['mika', 'jules', 'noah', 'sage', 'ada', 'lin', 'ellis', 'echo', 'karbowiak'],
])
const reserved = new Set([
  'chanserv',
  'nickserv',
  'histserv',
  'hostserv',
  'operserv',
  'botserv',
  'memoserv',
  'global',
  'ircord',
  'admin',
  'root',
])
function hashPassword(password: string) {
  const salt = randomBytes(16)
  return `${salt.toString('hex')}:${scryptSync(password, salt, 32).toString('hex')}`
}
function passwordMatches(password: string, stored: string) {
  const [salt, hash] = stored.split(':')
  const actual = scryptSync(password, Buffer.from(salt!, 'hex'), 32)
  return timingSafeEqual(actual, Buffer.from(hash!, 'hex'))
}
const passwordProblem = (password: unknown) =>
  typeof password !== 'string' || password.length < 8
    ? 'Use at least 8 characters for your password.'
    : password.length > 128
      ? 'Use 128 characters or fewer for your password.'
      : ''
/** Same rules and wording as the Go service's UsernameProblem. */
function usernameProblem(name: string) {
  if (!name) return 'Enter a username.'
  if (/\s/.test(name)) return 'Usernames can’t contain spaces. Your display name can.'
  if (nicknameError(name, 1000))
    return 'Use letters, numbers or IRC nickname symbols, starting with a letter or symbol.'
  if (name.length > 30) return 'Use 30 characters or fewer.'
  return ''
}

export async function usernameCheck(name: unknown) {
  const value = typeof name === 'string' ? name.trim() : ''
  const reason = usernameProblem(value)
  if (reason) return { available: false, reason }
  if (reserved.has(fold(value))) return { available: false, reason: `${value} is reserved.` }
  if (networkNames.has(fold(value)) || (await storage().hasItem(`auth:usernames:${fold(value)}`)))
    return { available: false, reason: `${value} is already taken.` }
  return { available: true }
}

export async function signup(event: H3Event, body: { username?: unknown; password?: unknown }) {
  await purgeExpired()
  const username = typeof body.username === 'string' ? body.username.trim() : ''
  const check = await usernameCheck(username)
  if (!check.available)
    throw check.reason?.includes('taken')
      ? fail(409, 'That username is already taken.')
      : fail(422, check.reason!)
  const problem = passwordProblem(body.password)
  if (problem) throw fail(422, problem)
  const now = new Date().toISOString()
  const account: StoredAccount = {
    id: randomUUID(),
    username,
    passwordHash: hashPassword(body.password as string),
    createdAt: now,
    lastSeenAt: now,
  }
  await storage().setItem(`auth:accounts:${account.id}`, account)
  await storage().setItem(`auth:usernames:${fold(username)}`, account.id)
  startSession(event, account.id)
  return { account: view(account) }
}

// A small per-process limiter: ten attempts per minute per address.
const attempts = new Map<string, { count: number; since: number }>()
export async function login(
  event: H3Event,
  body: { username?: unknown; password?: unknown } | undefined,
) {
  const ip = getRequestIP(event, { xForwardedFor: true }) ?? 'unknown'
  const entry = attempts.get(ip)
  if (entry && Date.now() - entry.since < 60_000 && entry.count >= 10)
    throw fail(429, 'Too many authentication attempts; try again shortly.')
  if (!entry || Date.now() - entry.since >= 60_000)
    attempts.set(ip, { count: 1, since: Date.now() })
  else entry.count++
  const username = typeof body?.username === 'string' ? body.username.trim() : ''
  const id = await storage().getItem<string>(`auth:usernames:${fold(username)}`)
  const account = id ? await storage().getItem<StoredAccount>(`auth:accounts:${id}`) : null
  if (
    !account ||
    typeof body?.password !== 'string' ||
    !passwordMatches(body.password, account.passwordHash)
  )
    throw fail(401, 'That username and password don’t match.')
  account.lastSeenAt = new Date().toISOString()
  await storage().setItem(`auth:accounts:${account.id}`, account)
  startSession(event, account.id)
  return { account: view(account) }
}

export async function changePassword(
  event: H3Event,
  body: { current?: unknown; password?: unknown },
) {
  const account = await requireAccount(event)
  const stored = (await storage().getItem<StoredAccount>(`auth:accounts:${account.id}`))!
  if (typeof body.current !== 'string' || !passwordMatches(body.current, stored.passwordHash))
    throw fail(403, 'Your current password is wrong.')
  const problem = passwordProblem(body.password)
  if (problem) throw fail(422, problem)
  stored.passwordHash = hashPassword(body.password as string)
  await storage().setItem(`auth:accounts:${stored.id}`, stored)
}

// ---- Sessions -------------------------------------------------------------------------------

/** The signed-in account, or null. Refreshes `lastSeenAt` at most once an hour. */
export function currentAccount(event: H3Event): Promise<Account | null> {
  return accountForToken(getCookie(event, 'ircord_session'))
}
/** Resolves a session cookie value; used directly by the WebSocket upgrade. */
export async function accountForToken(token: string | undefined): Promise<Account | null> {
  if (!token) return null
  const session = await storage().getItem<{ accountId: string }>(`auth:sessions:${sha256(token)}`)
  const account = session
    ? await storage().getItem<StoredAccount>(`auth:accounts:${session.accountId}`)
    : null
  if (!account?.username) return null
  if (Date.now() - new Date(account.lastSeenAt).getTime() > 3_600_000) {
    account.lastSeenAt = new Date().toISOString()
    await storage().setItem(`auth:accounts:${account.id}`, account)
  }
  return view(account)
}
export async function requireAccount(event: H3Event) {
  const account = await currentAccount(event)
  if (!account) throw fail(401, 'Sign in to continue.')
  return account
}

export async function logout(event: H3Event) {
  const token = getCookie(event, 'ircord_session')
  if (token) await storage().removeItem(`auth:sessions:${sha256(token)}`)
  deleteCookie(event, 'ircord_session', { path: '/' })
}

export async function deleteAccount(event: H3Event) {
  const account = await requireAccount(event)
  await removeAccount(account.id)
  await logout(event)
}
async function removeAccount(id: string) {
  const stored = await storage().getItem<StoredAccount>(`auth:accounts:${id}`)
  if (stored?.username) await storage().removeItem(`auth:usernames:${fold(stored.username)}`)
  await storage().removeItem(`auth:accounts:${id}`)
  for (const key of await storage().getKeys(`accounts:${id}`)) await storage().removeItem(key)
  // Sessions point at a missing account and simply stop resolving.
}
async function purgeExpired() {
  for (const key of await storage().getKeys('auth:accounts')) {
    const account = await storage().getItem<StoredAccount>(key)
    // Key accounts from before #64 have no username and are removed.
    if (
      account &&
      (!account.username || Date.now() - new Date(account.lastSeenAt).getTime() > RETENTION)
    )
      await removeAccount(account.id)
  }
}
