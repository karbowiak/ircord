import { createHash, randomBytes, randomUUID, scryptSync, timingSafeEqual } from 'node:crypto'
import { deleteCookie, getCookie, setCookie, type H3Event } from 'h3'
import type { Account } from '../../shared/types/chat'
import type { UsernameCheck } from '../../shared/types/auth'
import { nicknameError } from '../../shared/utils/nickname'

/*
 * Mock of the Go service's accounts. New accounts sign up with a username and password (#64);
 * the username is also their irc.ircord.com account and nick, so names already taken on the
 * network are refused. Older accounts are a secret key (only its SHA-256 is stored) and can
 * choose a username and password later. Logging in exchanges either for an opaque session
 * cookie. Accounts unused for 90 days are deleted.
 */
const DAY = 86_400_000
const RETENTION = 90 * DAY
const storage = () => useStorage('data')
const sha256 = (value: string) => createHash('sha256').update(value).digest('hex')
const alphabet = '0123456789ABCDEFGHJKMNPQRSTVWXYZ' // Crockford base32

/** ircord-XXXXX-XXXXX-XXXXX-XXXXX-XXXXX-XXXXX: 30 base32 characters, 150 bits. */
export function generateKey() {
  const bytes = randomBytes(30)
  const chars = [...bytes].map((byte) => alphabet[byte & 31]).join('')
  return `ircord-${chars.match(/.{5}/g)!.join('-')}`
}
export const normalizeKey = (key: unknown) =>
  typeof key === 'string'
    ? key
        .trim()
        .toUpperCase()
        .replace(/^IRCORD-/, 'ircord-')
    : ''

interface StoredAccount extends Omit<Account, 'expiresAt'> {
  keyHash?: string
  username?: string
  /** scrypt salt and hash, hex, `salt:hash`. */
  passwordHash?: string
}
const view = (account: StoredAccount): Account & { username?: string } => ({
  id: account.id,
  createdAt: account.createdAt,
  lastSeenAt: account.lastSeenAt,
  expiresAt: new Date(new Date(account.lastSeenAt).getTime() + RETENTION).toISOString(),
  ...(account.username ? { username: account.username } : {}),
})

// ---- Usernames and passwords ----------------------------------------------------------------

/** Nicks the demo network already has (its members and services). */
const networkNames = new Set(
  ['mika', 'jules', 'noah', 'sage', 'ada', 'lin', 'ellis', 'echo', 'karbowiak'].concat([
    'chanserv',
    'nickserv',
    'histserv',
    'operserv',
    'hostserv',
  ]),
)
const fold = (name: string) => name.toLowerCase()
function hashPassword(password: string) {
  const salt = randomBytes(16)
  return `${salt.toString('hex')}:${scryptSync(password, salt, 32).toString('hex')}`
}
function passwordMatches(password: string, stored: string) {
  const [salt, hash] = stored.split(':')
  const actual = scryptSync(password, Buffer.from(salt!, 'hex'), 32)
  return timingSafeEqual(actual, Buffer.from(hash!, 'hex'))
}
const passwordError = (password: unknown) =>
  typeof password !== 'string' || password.length < 8
    ? 'Use at least 8 characters for your password.'
    : password.length > 128
      ? 'Use 128 characters or fewer for your password.'
      : ''

export async function usernameCheck(name: unknown): Promise<UsernameCheck> {
  const value = typeof name === 'string' ? name.trim() : ''
  const problem = nicknameError(value, 30).replace(/nickname/g, 'username')
  if (problem) return { available: false, reason: problem }
  if (networkNames.has(fold(value)) || (await storage().hasItem(`auth:usernames:${fold(value)}`)))
    return { available: false, reason: `${value} is already taken on irc.ircord.com.` }
  return { available: true }
}
async function claimUsername(username: unknown, password: unknown) {
  const check = await usernameCheck(username)
  if (!check.available)
    throw createError({
      statusCode: check.reason?.includes('taken') ? 409 : 422,
      statusMessage: check.reason,
    })
  const problem = passwordError(password)
  if (problem) throw createError({ statusCode: 422, statusMessage: problem })
  return { username: (username as string).trim(), passwordHash: hashPassword(password as string) }
}

export async function signup(event: H3Event, body: { username?: unknown; password?: unknown }) {
  await purgeExpired()
  const { username, passwordHash } = await claimUsername(body.username, body.password)
  const now = new Date().toISOString()
  const account: StoredAccount = {
    id: randomUUID(),
    username,
    passwordHash,
    createdAt: now,
    lastSeenAt: now,
  }
  await storage().setItem(`auth:accounts:${account.id}`, account)
  await storage().setItem(`auth:usernames:${fold(username)}`, account.id)
  startSession(event, account.id)
  return { account: view(account) }
}

/** A key account chooses its username and password; the key keeps working until retired. */
export async function setCredentials(
  event: H3Event,
  body: { username?: unknown; password?: unknown },
) {
  const current = await requireAccount(event)
  const stored = (await storage().getItem<StoredAccount>(`auth:accounts:${current.id}`))!
  if (stored.username)
    throw createError({ statusCode: 409, statusMessage: 'This account already has a username.' })
  const { username, passwordHash } = await claimUsername(body.username, body.password)
  Object.assign(stored, { username, passwordHash })
  await storage().setItem(`auth:accounts:${stored.id}`, stored)
  await storage().setItem(`auth:usernames:${fold(username)}`, stored.id)
  return { account: view(stored) }
}

export async function changePassword(
  event: H3Event,
  body: { current?: unknown; password?: unknown },
) {
  const account = await requireAccount(event)
  const stored = (await storage().getItem<StoredAccount>(`auth:accounts:${account.id}`))!
  if (
    !stored.passwordHash ||
    typeof body.current !== 'string' ||
    !passwordMatches(body.current, stored.passwordHash)
  )
    throw createError({ statusCode: 403, statusMessage: 'Your current password is wrong.' })
  const problem = passwordError(body.password)
  if (problem) throw createError({ statusCode: 422, statusMessage: problem })
  stored.passwordHash = hashPassword(body.password as string)
  await storage().setItem(`auth:accounts:${stored.id}`, stored)
}

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

export async function register(event: H3Event) {
  await purgeExpired()
  const key = generateKey()
  const now = new Date().toISOString()
  const account: StoredAccount = {
    id: randomUUID(),
    keyHash: sha256(key),
    createdAt: now,
    lastSeenAt: now,
  }
  await storage().setItem(`auth:accounts:${account.id}`, account)
  await storage().setItem(`auth:keys:${account.keyHash}`, account.id)
  startSession(event, account.id)
  return { key, account: view(account) }
}

// A small per-process limiter: ten attempts per minute per address.
const attempts = new Map<string, { count: number; since: number }>()
export async function login(
  event: H3Event,
  body: { key?: unknown; username?: unknown; password?: unknown } | undefined,
) {
  const ip = getRequestIP(event, { xForwardedFor: true }) ?? 'unknown'
  const entry = attempts.get(ip)
  if (entry && Date.now() - entry.since < 60_000 && entry.count >= 10)
    throw createError({
      statusCode: 429,
      statusMessage: 'Too many attempts. Wait a minute and try again.',
    })
  if (!entry || Date.now() - entry.since >= 60_000)
    attempts.set(ip, { count: 1, since: Date.now() })
  else entry.count++
  const byPassword = typeof body?.username === 'string'
  const id = byPassword
    ? await storage().getItem<string>(`auth:usernames:${fold((body!.username as string).trim())}`)
    : await storage().getItem<string>(`auth:keys:${sha256(normalizeKey(body?.key))}`)
  const account = id ? await storage().getItem<StoredAccount>(`auth:accounts:${id}`) : null
  const valid = byPassword
    ? !!account?.passwordHash &&
      typeof body?.password === 'string' &&
      passwordMatches(body.password, account.passwordHash)
    : !!account
  if (!account || !valid)
    throw createError({
      statusCode: 401,
      statusMessage: byPassword
        ? 'That username and password don’t match.'
        : 'That key doesn’t match an account.',
    })
  account.lastSeenAt = new Date().toISOString()
  await storage().setItem(`auth:accounts:${account.id}`, account)
  startSession(event, account.id)
  return { account: view(account) }
}

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
  if (!account) return null
  if (Date.now() - new Date(account.lastSeenAt).getTime() > 3_600_000) {
    account.lastSeenAt = new Date().toISOString()
    await storage().setItem(`auth:accounts:${account.id}`, account)
  }
  return view(account)
}
export async function requireAccount(event: H3Event) {
  const account = await currentAccount(event)
  if (!account) throw createError({ statusCode: 401, statusMessage: 'Sign in to continue.' })
  return account
}

export async function logout(event: H3Event) {
  const token = getCookie(event, 'ircord_session')
  if (token) await storage().removeItem(`auth:sessions:${sha256(token)}`)
  deleteCookie(event, 'ircord_session', { path: '/' })
}

export async function rotate(event: H3Event) {
  const account = await requireAccount(event)
  const stored = (await storage().getItem<StoredAccount>(`auth:accounts:${account.id}`))!
  if (stored.keyHash) await storage().removeItem(`auth:keys:${stored.keyHash}`)
  const key = generateKey()
  stored.keyHash = sha256(key)
  await storage().setItem(`auth:accounts:${account.id}`, stored)
  await storage().setItem(`auth:keys:${stored.keyHash}`, account.id)
  return { key }
}

export async function deleteAccount(event: H3Event) {
  const account = await requireAccount(event)
  await removeAccount(account.id)
  await logout(event)
}
async function removeAccount(id: string) {
  const stored = await storage().getItem<StoredAccount>(`auth:accounts:${id}`)
  if (stored?.keyHash) await storage().removeItem(`auth:keys:${stored.keyHash}`)
  if (stored?.username) await storage().removeItem(`auth:usernames:${fold(stored.username)}`)
  await storage().removeItem(`auth:accounts:${id}`)
  for (const key of await storage().getKeys(`accounts:${id}`)) await storage().removeItem(key)
  // Sessions point at a missing account and simply stop resolving.
}
async function purgeExpired() {
  for (const key of await storage().getKeys('auth:accounts')) {
    const account = await storage().getItem<StoredAccount>(key)
    if (account && Date.now() - new Date(account.lastSeenAt).getTime() > RETENTION)
      await removeAccount(account.id)
  }
}
