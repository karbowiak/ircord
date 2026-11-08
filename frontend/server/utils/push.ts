import * as webPush from 'web-push'
import type { Conversation, Message } from '../../shared/types/chat'
import {
  defaultNotificationSettings,
  notificationFor,
  sanitizeNotificationSettings,
  type NotificationSettings,
} from '../../shared/notifications'
import { stripFormatting } from '../../shared/utils/irc'
import { recipientIn } from '../../shared/group-mentions'
import type { DemoSession } from './demo-store'

/*
 * Mock of the backend's notification rules and Web Push (BACKEND-REQUESTS.md §10). Pushes are
 * real: the browser's push service delivers them even when no IRCord window is open.
 */
const storage = () => useStorage('data')

// ---- Rules ------------------------------------------------------------------------------------

export async function readNotificationSettings(account: string): Promise<NotificationSettings> {
  const stored = await storage().getItem(`accounts:${account}:notifications`)
  return stored ? sanitizeNotificationSettings(stored) : defaultNotificationSettings
}
export async function writeNotificationSettings(account: string, input: unknown) {
  const value = sanitizeNotificationSettings(input)
  await storage().setItem(`accounts:${account}:notifications`, value)
  return value
}

// ---- VAPID and subscriptions ------------------------------------------------------------------

interface StoredSubscription {
  endpoint: string
  keys: { p256dh: string; auth: string }
  createdAt: string
  device?: string
  /** The signed-in device (currentDevice) that registered it. */
  session?: string
}
let vapid: Promise<{ publicKey: string; privateKey: string }> | undefined
/** One key pair per installation, generated on first use and kept in storage. */
export function vapidKeys() {
  vapid ??= (async () => {
    const existing = await storage().getItem<{ publicKey: string; privateKey: string }>(
      'push:vapid',
    )
    if (existing) return existing
    const keys = webPush.generateVAPIDKeys()
    await storage().setItem('push:vapid', keys)
    return keys
  })()
  return vapid
}

const pushHosts =
  /^https:\/\/([\w-]+\.)*(googleapis\.com|mozilla\.com|mozaws\.net|push\.apple\.com|notify\.windows\.com)\//
/** Only real browser push services, so the server can't be used to make arbitrary requests. */
export function validSubscription(value: unknown): value is StoredSubscription {
  const v = value as StoredSubscription
  return (
    !!v &&
    typeof v.endpoint === 'string' &&
    v.endpoint.length < 2048 &&
    pushHosts.test(v.endpoint) &&
    typeof v.keys?.p256dh === 'string' &&
    v.keys.p256dh.length < 200 &&
    typeof v.keys?.auth === 'string' &&
    v.keys.auth.length < 100
  )
}
const subscriptionsKey = (account: string) => `accounts:${account}:push`
export async function listSubscriptions(account: string) {
  return (await storage().getItem<StoredSubscription[]>(subscriptionsKey(account))) ?? []
}
export async function addSubscription(
  account: string,
  input: StoredSubscription,
  device?: string,
  session?: string,
) {
  const list = (await listSubscriptions(account)).filter((s) => s.endpoint !== input.endpoint)
  list.push({
    endpoint: input.endpoint,
    keys: { p256dh: input.keys.p256dh, auth: input.keys.auth },
    createdAt: new Date().toISOString(),
    device: device?.slice(0, 80),
    session,
  })
  await storage().setItem(subscriptionsKey(account), list.slice(-10))
}
export async function removeSubscription(account: string, endpoint: string) {
  const list = await listSubscriptions(account)
  await storage().setItem(
    subscriptionsKey(account),
    list.filter((s) => s.endpoint !== endpoint),
  )
}

/** A signed-out device's registrations go with it, like the backend's. */
export async function removeSessionSubscriptions(account: string, session: string) {
  if (!session) return
  const list = await listSubscriptions(account)
  await storage().setItem(
    subscriptionsKey(account),
    list.filter((s) => s.session !== session),
  )
}

export interface PushPayload {
  title: string
  body: string
  conversationId?: string
  tag?: string
  /** Unread mentions and direct messages, for the app icon badge. */
  badge?: number
}
/**
 * Pushes to every device of the account, or only `session`'s. Resolves how many the push
 * service accepted, and how many failed for another reason than being gone.
 */
export async function sendPush(account: string, payload: PushPayload, session?: string) {
  let delivered = 0
  let failed = 0
  const keys = await vapidKeys()
  const subject = process.env.NUXT_PUSH_SUBJECT || 'mailto:admin@ircord.invalid'
  const targets = (await listSubscriptions(account)).filter(
    (s) => session === undefined || s.session === session,
  )
  for (const subscription of targets) {
    try {
      await webPush.sendNotification(subscription, JSON.stringify(payload), {
        vapidDetails: { subject, publicKey: keys.publicKey, privateKey: keys.privateKey },
        TTL: 60 * 60,
        urgency: 'high',
        topic: payload.tag?.replace(/[^\w-]/g, '').slice(0, 32),
      })
      delivered++
    } catch (error) {
      // Gone: the user revoked permission or uninstalled the app.
      const status = (error as { statusCode?: number }).statusCode
      if (status === 404 || status === 410) await removeSubscription(account, subscription.endpoint)
      else failed++
    }
  }
  return { delivered, failed }
}

/** Called for every new message; pushes when it matches the account's rules. */
export async function notifyIncoming(session: DemoSession, message: Message) {
  const c: Conversation | undefined = session.bootstrap.conversations.find(
    (x) => x.id === message.conversationId,
  )
  if (!c || message.senderId === 'you') return
  const subscriptions = await listSubscriptions(session.id)
  if (!subscriptions.length) return
  const settings = await readNotificationSettings(session.id)
  const network = session.bootstrap.networks.find((n) => n.id === c.networkId)
  const sender = session.bootstrap.members.find((m) => m.id === message.senderId)
  const you = recipientIn(c, network, session.bootstrap.user, message)
  const kind = notificationFor(settings, c, message, you, sender?.nick)
  if (!kind) return
  const badge = session.bootstrap.conversations.reduce(
    (sum, x) => sum + (x.kind === 'direct' ? x.unreadCount : x.mentionCount),
    0,
  )
  await sendPush(session.id, {
    title:
      c.kind === 'channel'
        ? `${sender?.nick ?? 'Someone'} in #${c.name}`
        : (sender?.displayName ?? c.name),
    body: message.gif
      ? 'Sent a GIF'
      : message.kind === 'action'
        ? `* ${sender?.nick} ${stripFormatting(message.text)}`
        : stripFormatting(message.text).slice(0, 240),
    conversationId: c.id,
    tag: c.id,
    badge,
  })
}
