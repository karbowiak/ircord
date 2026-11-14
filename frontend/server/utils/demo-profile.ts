import { emit, getNetwork } from './demo-store'
import { randomUUID } from 'node:crypto'
import type { H3Event } from 'h3'
export async function updateDemoProfile(event: H3Event, networkId?: string) {
  const session = await demoSession(event)
  const input = await readBody(event)
  // The disposable demo uses the same status patch limits as the Go backend.
  // Validate before changing any field so a rejected patch has no side effects.
  for (const [key, limit] of [
    ['statusText', 200],
    ['statusEmoji', 64],
  ] as const) {
    const value = input?.[key]
    if (
      value !== undefined &&
      value !== null &&
      (typeof value !== 'string' ||
        new TextEncoder().encode(value).length > limit ||
        /\p{Cc}/u.test(value))
    )
      throw createError({ statusCode: 400, statusMessage: 'Invalid status text or emoji.' })
  }
  const deadline = input?.statusExpiresAt
  if (
    deadline !== undefined &&
    deadline !== null &&
    deadline !== '' &&
    (typeof deadline !== 'string' ||
      deadline.length > 64 ||
      !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(deadline) ||
      !Number.isFinite(Date.parse(deadline)))
  )
    throw createError({ statusCode: 400, statusMessage: 'Invalid status expiration.' })
  if (
    !input ||
    typeof input !== 'object' ||
    Array.isArray(input) ||
    (input.displayName !== undefined &&
      input.displayName !== null &&
      (typeof input.displayName !== 'string' || input.displayName.length > 40)) ||
    (input.bio !== undefined &&
      (typeof input.bio !== 'string' ||
        input.bio.replaceAll('\r\n', '\n').length > 300 ||
        /\r(?!\n)|\0/.test(input.bio)))
  )
    throw createError({
      statusCode: 400,
      statusMessage: 'Use a display name up to 40 characters and a bio up to 300 characters.',
    })
  const network = networkId ? getNetwork(session, networkId) : undefined
  const member = network ? network.profile! : session.bootstrap.user
  if (
    input.avatarUrl !== undefined &&
    input.avatarUrl !== null &&
    typeof input.avatarUrl !== 'string'
  )
    throw createError({ statusCode: 400, statusMessage: 'avatar must be an HTTPS URL' })
  if (
    input.nameColor !== undefined &&
    input.nameColor !== null &&
    (typeof input.nameColor !== 'string' ||
      (input.nameColor !== '' && !/^#[\da-f]{6}$/i.test(input.nameColor)))
  )
    throw createError({
      statusCode: 400,
      statusMessage: 'Choose a valid name color or use automatic coloring.',
    })
  let avatar: typeof session.profileAvatar
  if (input.avatarDataUrl !== undefined && input.avatarDataUrl !== null) {
    if (typeof input.avatarDataUrl !== 'string' || input.avatarDataUrl.length > 180_000)
      throw createError({
        statusCode: 400,
        statusMessage: 'Your profile image is too large. Choose a smaller image.',
      })
    const match = /^data:(image\/(?:png|jpeg|webp));base64,([A-Za-z0-9+/]+={0,2})$/.exec(
      input.avatarDataUrl,
    )
    if (!match)
      throw createError({
        statusCode: 400,
        statusMessage: 'Choose a PNG, JPEG or WebP profile image.',
      })
    const bytes = Buffer.from(match[2]!, 'base64')
    if (!bytes.length || bytes.length > 128 * 1024)
      throw createError({ statusCode: 400, statusMessage: 'Your profile image is too large.' })
    avatar = { bytes, contentType: match[1]!, revision: randomUUID() }
  }
  // An avatar URL (a default avatar on this origin) stands as given. The Go backend accepts only
  // HTTPS here; the demo also takes its own origin because it is served over plain http.
  let avatarUrl: string | undefined
  if (typeof input.avatarUrl === 'string' && input.avatarUrl && input.avatarDataUrl === undefined) {
    let url: URL | undefined
    try {
      url = new URL(input.avatarUrl)
    } catch {
      // An unparsable URL stays undefined and is rejected below.
    }
    const origin = getRequestURL(event).origin
    if (
      !url ||
      (url.protocol !== 'https:' && url.origin !== origin) ||
      url.username ||
      input.avatarUrl.length > 300 ||
      /[\r\n\0]/.test(input.avatarUrl)
    )
      throw createError({ statusCode: 400, statusMessage: 'avatar must be an HTTPS URL' })
    avatarUrl = input.avatarUrl
  }
  // Omitted keeps the current name; empty or null clears it back to the nickname.
  if (input.displayName !== undefined) {
    const name = input.displayName?.trim()
    if (name) Object.assign(member, { displayName: name, nameSource: 'profile' })
    else if (member.realname?.trim() && member.realname !== 'IRCord')
      Object.assign(member, { displayName: member.realname, nameSource: 'realname' })
    else Object.assign(member, { displayName: member.nick, nameSource: 'nick' })
  }
  if (input.bio !== undefined) member.bio = input.bio.replaceAll('\r\n', '\n')
  if (input.nameColor === null || input.nameColor === '') delete member.nameColor
  else if (input.nameColor !== undefined) member.nameColor = input.nameColor
  if (input.avatarDataUrl === null) {
    if (networkId) delete session.networkAvatars[networkId]
    else delete session.profileAvatar
    delete member.avatarUrl
  } else if (avatar) {
    if (networkId) session.networkAvatars[networkId] = avatar
    else session.profileAvatar = avatar
    member.avatarUrl = networkId
      ? `/api/v1/networks/${encodeURIComponent(networkId)}/profile/avatar?revision=${avatar.revision}`
      : `/api/v1/profile/avatar?revision=${avatar.revision}`
  } else if (
    avatarUrl !== undefined ||
    (input.avatarUrl === '' && input.avatarDataUrl === undefined)
  ) {
    if (networkId) delete session.networkAvatars[networkId]
    else delete session.profileAvatar
    if (avatarUrl) member.avatarUrl = avatarUrl
    else delete member.avatarUrl
  }
  for (const key of ['statusText', 'statusEmoji'] as const) {
    if (input[key] === null || input[key] === '') delete member[key]
    else if (input[key] !== undefined) member[key] = input[key]
  }
  if (deadline === null || deadline === '') delete member.statusExpiresAt
  else if (deadline !== undefined) member.statusExpiresAt = deadline
  else if ('statusText' in input || 'statusEmoji' in input) delete member.statusExpiresAt
  if (network) emit(session, { type: 'network.updated', data: network })
  else emit(session, { type: 'profile.updated', data: member })
  return member
}
