import { createHash, randomBytes, randomUUID } from 'node:crypto'
import { deleteCookie, getCookie, setCookie, type H3Event } from 'h3'
import type { Account } from '../../shared/types/chat'

/*
 * Mock of the Go service's anonymous key accounts (BACKEND-REQUESTS.md §1). An account is a
 * secret key; only its SHA-256 is stored. Logging in exchanges the key for an opaque session
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
  keyHash: string
}
const view = (account: StoredAccount): Account => ({
  id: account.id,
  createdAt: account.createdAt,
  lastSeenAt: account.lastSeenAt,
  expiresAt: new Date(new Date(account.lastSeenAt).getTime() + RETENTION).toISOString(),
})

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
export async function login(event: H3Event, key: unknown) {
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
  const id = await storage().getItem<string>(`auth:keys:${sha256(normalizeKey(key))}`)
  const account = id ? await storage().getItem<StoredAccount>(`auth:accounts:${id}`) : null
  if (!account)
    throw createError({ statusCode: 401, statusMessage: 'That key doesn’t match an account.' })
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
  await storage().removeItem(`auth:keys:${stored.keyHash}`)
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
  if (stored) await storage().removeItem(`auth:keys:${stored.keyHash}`)
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
