import type { ChannelMember, ChannelMode, User } from '~/types'

export type UsersById = Record<string, User>

export function buildUsersById(users: User[]): UsersById {
  return Object.fromEntries(users.map(user => [user.id, user]))
}

export function createMemberFactory(usersById: UsersById) {
  return (userId: string, mode: ChannelMode): ChannelMember => {
    const user = usersById[userId]
    if (!user) {
      throw new Error(`Missing mock user with id ${userId}`)
    }

    return { user, mode }
  }
}
