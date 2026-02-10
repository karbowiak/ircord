import type { DMChannel, Message, Server, User } from '~/types'
import { buildMockDms } from '~/mocks/chat/dms'
import { buildMockMessages } from '~/mocks/chat/messages'
import { buildMockServers } from '~/mocks/chat/servers'
import { buildUsersById, createMemberFactory } from '~/mocks/chat/helpers'
import { mockUsers } from '~/mocks/chat/users'

export type MockChatData = {
  users: User[]
  servers: Server[]
  dms: DMChannel[]
  messages: Message[]
}

export function createMockChatData(): MockChatData {
  const users = [...mockUsers]
  const usersById = buildUsersById(users)
  const member = createMemberFactory(usersById)

  return {
    users,
    servers: buildMockServers(member),
    dms: buildMockDms(usersById),
    messages: buildMockMessages(usersById),
  }
}
