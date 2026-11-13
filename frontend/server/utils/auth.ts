import { createHash, randomBytes, randomUUID, scryptSync, timingSafeEqual } from 'node:crypto'
import { deleteCookie, getCookie, setCookie, type H3Event } from 'h3'
import type { Account } from '../../shared/types/chat'
import type { components } from '../../shared/generated/api'
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
  ...(account.onboardingNetworkId ? { onboardingNetworkId: account.onboardingNetworkId } : {}),
  createdAt: account.createdAt,
  lastSeenAt: account.lastSeenAt,
  expiresAt: new Date(new Date(account.lastSeenAt).getTime() + RETENTION).toISOString(),
})
const fail = (statusCode: number, statusMessage: string) =>
  createError({ statusCode, statusMessage })

/**
 * A signed-in browser: the Go service's session row, with its device metadata (#85). The
 * record is written once; last-seen times live under their own `auth:seen:` key, so a request
 * refreshing them can never write back (resurrect) a session that was signed out meanwhile.
 */
interface StoredSession {
  accountId: string
  createdAt: number
  lastSeenAt?: number
  userAgent?: string
}
const seenKey = (sessionKey: string) => `auth:seen:${sessionKey.split(':').pop()}`
const devicePreferencesKey = (account: string, id: string) => `accounts:${account}:devices:${id}`
function startSession(event: H3Event, accountId: string) {
  const token = randomBytes(32).toString('base64url')
  const session: StoredSession = {
    accountId,
    createdAt: Date.now(),
    lastSeenAt: Date.now(),
    userAgent: (getHeader(event, 'user-agent') ?? '').slice(0, 256),
  }
  void storage().setItem(`auth:sessions:${sha256(token)}`, session)
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
  // The demo serves browsers directly, so it ignores forwarded addresses.
  const ip = peerAddress(event) ?? 'unknown'
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

/**
 * The signed-in account, or null. Refreshes the account's `lastSeenAt` at most once an hour
 * and the session's last-seen time at most once a minute.
 */
export function currentAccount(event: H3Event): Promise<Account | null> {
  return accountForToken(getCookie(event, 'ircord_session'))
}
/** Resolves a session cookie value; used directly by the WebSocket upgrade. */
export async function accountForToken(token: string | undefined): Promise<Account | null> {
  if (!token) return null
  const key = `auth:sessions:${sha256(token)}`
  const session = await storage().getItem<StoredSession>(key)
  if (session) {
    const seen = (await storage().getItem<number>(seenKey(key))) ?? session.lastSeenAt ?? 0
    if (Date.now() - seen > 60_000) await storage().setItem(seenKey(key), Date.now())
  }
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
  const account = await currentAccount(event)
  if (account && token) {
    const id = currentDevice(event)
    await removeSessionSubscriptions(account.id, id)
    await storage().removeItem(devicePreferencesKey(account.id, id))
  }
  if (token) {
    await storage().removeItem(`auth:sessions:${sha256(token)}`)
    await storage().removeItem(seenKey(`auth:sessions:${sha256(token)}`))
  }
  deleteCookie(event, 'ircord_session', { path: '/' })
}

export async function deleteAccount(event: H3Event, body: { password?: unknown }) {
  const account = await requireAccount(event)
  const stored = (await storage().getItem<StoredAccount>(`auth:accounts:${account.id}`))!
  if (typeof body.password !== 'string' || !passwordMatches(body.password, stored.passwordHash))
    throw fail(403, 'Your current password is wrong.')
  await removeAccount(account.id)
  await logout(event)
}
export async function completeOnboarding(event: H3Event) {
  const account = await requireAccount(event)
  const stored = await storage().getItem<StoredAccount>(`auth:accounts:${account.id}`)
  if (!stored?.onboardingNetworkId) return
  delete stored.onboardingNetworkId
  await storage().setItem(`auth:accounts:${stored.id}`, stored)
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

// ---- Devices & sessions (#85) ---------------------------------------------------------------

const deviceId = (key: string) => sha256(`ircord-device:${key}`).slice(0, 32)
/** The requesting signed-in device's id, as the device list shows it ('' when signed out). */
export function currentDevice(event: H3Event) {
  const token = getCookie(event, 'ircord_session')
  return token ? deviceId(`auth:sessions:${sha256(token)}`) : ''
}
/** Same rules as the Go service's DeviceLabel. */
export function deviceLabel(ua: string) {
  const has = (s: string) => ua.includes(s)
  const browser =
    has('Edg/') || has('EdgA/') || has('EdgiOS/')
      ? 'Edge'
      : has('OPR/')
        ? 'Opera'
        : has('Firefox/') || has('FxiOS/')
          ? 'Firefox'
          : has('Chrome/') || has('CriOS/') || has('Chromium/')
            ? 'Chrome'
            : has('Safari/') && has('Version/')
              ? 'Safari'
              : ''
  const os =
    has('iPhone') || has('iPod')
      ? 'iOS'
      : has('iPad')
        ? 'iPadOS'
        : has('Android')
          ? 'Android'
          : has('Windows')
            ? 'Windows'
            : has('CrOS')
              ? 'ChromeOS'
              : has('Macintosh') || has('Mac OS X')
                ? 'macOS'
                : has('Linux')
                  ? 'Linux'
                  : ''
  return { browser, os }
}
async function ownSessions(event: H3Event) {
  const account = await requireAccount(event)
  const token = getCookie(event, 'ircord_session')
  const mine = token ? `auth:sessions:${sha256(token)}` : ''
  const out: { key: string; session: StoredSession; current: boolean }[] = []
  for (const key of await storage().getKeys('auth:sessions')) {
    const session = await storage().getItem<StoredSession>(key)
    if (session?.accountId !== account.id) continue
    const seen = await storage().getItem<number>(seenKey(key))
    out.push({
      key,
      session: { ...session, lastSeenAt: Math.max(seen ?? 0, session.lastSeenAt ?? 0) || undefined },
      current: key === mine,
    })
  }
  return out.sort((a, b) => b.session.createdAt - a.session.createdAt)
}
/**
 * The demo has no live-connection tracking: status comes from the last-seen time alone
 * (recent, offline, or unknown when never recorded). Push is registered when the device
 * subscribed through the demo's Web Push.
 */
export async function listDevices(event: H3Event) {
  const account = await requireAccount(event)
  const pushed = new Set((await listSubscriptions(account.id)).map((s) => s.session))
  return {
    devices: await Promise.all((await ownSessions(event)).map(async ({ key, session, current }) => {
      const seen = session.lastSeenAt
      const id = deviceId(key)
      const saved = await storage().getItem<DemoDevicePreferences>(devicePreferencesKey(account.id, id))
      return {
        id,
        current,
        ...deviceLabel(session.userAgent ?? ''),
        name: saved?.name ?? '',
        formFactor: 'unknown' as const,
        focal: false,
        active: false,
        notifications: {
          content: saved?.content ?? 'inherit',
          delivery: saved?.delivery ?? 'off',
          sound: saved?.sound ?? 'mentions',
          preview: saved?.preview ?? true,
          initialized: !!saved,
          explicit: saved?.explicit ?? false,
          revision: saved?.revision ?? 0,
          permission: 'unknown' as const,
          subscription: pushed.has(id),
        },
        signedInAt: new Date(session.createdAt).toISOString(),
        ...(seen ? { lastSeenAt: new Date(seen).toISOString() } : {}),
        expiresAt: new Date(session.createdAt + RETENTION).toISOString(),
        status: !seen ? 'unknown' : Date.now() - seen < 5 * 60_000 ? 'recent' : 'offline',
        push: pushed.has(id) ? 'registered' : 'none',
      }
    })),
  }
}

type DevicePreferences = components['schemas']['DevicePreferences']
type DemoDevicePreferences = Omit<DevicePreferences, 'migrate'> & { explicit: boolean }
const preferenceWrites = new Map<string, Promise<void>>()

/** Demo preferences follow the session and reject stale edits, like the Go service. */
export async function saveDevicePreferences(event: H3Event, id: string, input: DevicePreferences) {
  const account = await requireAccount(event)
  const key = devicePreferencesKey(account.id, id)
  const previous = preferenceWrites.get(key)
  let release!: () => void
  const next = new Promise<void>((resolve) => (release = resolve))
  preferenceWrites.set(key, next)
  try {
    await previous
    const match = (await ownSessions(event)).find(({ key }) => deviceId(key) === id)
    if (!match) throw fail(404, 'That device is already signed out.')
    const saved = await storage().getItem<DemoDevicePreferences>(key)
    if (!input || input.revision !== (saved?.revision ?? 0))
      throw fail(409, 'Device settings changed. Try again.')
    if (input.migrate && !match.current) throw fail(403, 'Only this device can import its settings.')
    if (input.migrate && saved) throw fail(409, 'Device settings changed. Try again.')
    if (
      typeof input.name !== 'string' || input.name.length > 80 || /[\r\n\0]/.test(input.name) ||
      !['inherit', 'all', 'mentions', 'none'].includes(input.content) ||
      !['off', 'open', 'push'].includes(input.delivery) ||
      !['off', 'mentions', 'all'].includes(input.sound) ||
      typeof input.preview !== 'boolean'
    ) throw fail(400, 'Invalid device settings.')
    await storage().setItem(key, {
      name: input.name.trim(), content: input.content, delivery: input.delivery,
      sound: input.sound, preview: input.preview, revision: input.revision + 1,
      explicit: !input.migrate,
    } satisfies DemoDevicePreferences)
    return (await listDevices(event)).devices.find((device) => device.id === id)!
  } finally {
    release()
    if (preferenceWrites.get(key) === next) preferenceWrites.delete(key)
  }
}
export async function signOutDevice(event: H3Event, id: string) {
  const match = (await ownSessions(event)).find(({ key }) => deviceId(key) === id)
  if (!match) throw fail(404, 'That device is already signed out.')
  await removeSessionSubscriptions(match.session.accountId, id)
  await storage().removeItem(devicePreferencesKey(match.session.accountId, id))
  await storage().removeItem(match.key)
  await storage().removeItem(seenKey(match.key))
  if (match.current) deleteCookie(event, 'ircord_session', { path: '/' })
}
