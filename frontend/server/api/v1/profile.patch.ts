import { emit } from '../../utils/demo-store'
import { randomUUID } from 'node:crypto'
export default defineEventHandler(async (event) => {
  const session = await demoSession(event)
  const input = await readBody(event)
  if (
    !input ||
    (input.displayName !== undefined &&
      input.displayName !== null &&
      (typeof input.displayName !== 'string' || input.displayName.length > 40)) ||
    typeof input.bio !== 'string' ||
    input.bio.length > 300
  )
    throw createError({
      statusCode: 400,
      statusMessage: 'Use a display name up to 40 characters and a bio up to 300 characters.',
    })
  const member = session.bootstrap.user
  if (
    input.nameColor !== undefined &&
    input.nameColor !== null &&
    (typeof input.nameColor !== 'string' || !/^#[\da-f]{6}$/i.test(input.nameColor))
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
  // Omitted keeps the current name; empty or null clears it back to the nickname.
  if (input.displayName !== undefined) {
    const name = input.displayName?.trim()
    if (name) Object.assign(member, { displayName: name, nameSource: 'profile' })
    else Object.assign(member, { displayName: member.nick, nameSource: 'nick' })
  }
  member.bio = input.bio
  if (input.nameColor === null) delete member.nameColor
  else if (input.nameColor !== undefined) member.nameColor = input.nameColor.toLowerCase()
  if (input.avatarDataUrl === null) {
    delete session.profileAvatar
    delete member.avatarUrl
  } else if (avatar) {
    session.profileAvatar = avatar
    member.avatarUrl = `/api/v1/profile/avatar?revision=${avatar.revision}`
  }
  emit(session, { type: 'profile.updated', data: member })
  return member
})
