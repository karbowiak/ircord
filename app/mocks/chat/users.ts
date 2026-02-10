import type { User } from '~/types'

export const mockUsers: User[] = [
  { id: '1', username: 'karbowiak', avatar: 'K', status: 'online' },
  { id: '2', username: 'alice', avatar: 'A', status: 'online' },
  { id: '3', username: 'bob', avatar: 'B', status: 'away' },
  { id: '4', username: 'charlie', avatar: 'C', status: 'offline' },
  { id: '5', username: 'diana', avatar: 'D', status: 'online' },
  { id: '6', username: 'ChanServ', avatar: 'S', status: 'online' },
  { id: '7', username: 'NickServ', avatar: 'S', status: 'online' },
  { id: '8', username: 'evilpete', avatar: 'E', status: 'online' },
  { id: '9', username: 'rustacean', avatar: 'R', status: 'online' },
  { id: '10', username: 'kernel_hacker', avatar: 'H', status: 'away' },
]
