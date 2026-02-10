import type { DMChannel } from '~/types'
import type { UsersById } from '~/mocks/chat/helpers'

export function buildMockDms(usersById: UsersById): DMChannel[] {
  return [
    { id: 'dm1', recipient: usersById['2'], unread: true },
    { id: 'dm2', recipient: usersById['3'] },
    { id: 'dm3', recipient: usersById['5'], unread: true },
  ]
}
