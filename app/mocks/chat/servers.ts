import type { Server } from '~/types'
import type { createMemberFactory } from '~/mocks/chat/helpers'

type MemberFactory = ReturnType<typeof createMemberFactory>

export function buildMockServers(member: MemberFactory): Server[] {
  return [
    {
      id: 'libera',
      name: 'Libera Chat',
      abbreviation: 'LC',
      channels: [
        {
          id: 'libera-linux',
          name: 'linux',
          type: 'text',
          serverId: 'libera',
          topic: 'Linux support and discussion | Kernel 6.12 released | Be nice',
          modes: '+nt',
          members: [
            member('6', 'op'),
            member('1', 'op'),
            member('2', 'voice'),
            member('5', 'voice'),
            member('3', 'regular'),
            member('10', 'regular'),
            member('8', 'regular'),
          ],
        },
        {
          id: 'libera-programming',
          name: 'programming',
          type: 'text',
          serverId: 'libera',
          topic: 'General programming talk | No paste floods, use a pastebin',
          modes: '+nt',
          unread: true,
          members: [
            member('6', 'op'),
            member('2', 'op'),
            member('1', 'voice'),
            member('4', 'regular'),
            member('9', 'regular'),
          ],
        },
        {
          id: 'libera-rust',
          name: 'rust',
          type: 'text',
          serverId: 'libera',
          topic: 'Rust programming language | Edition 2024 is here | cargo is your friend',
          modes: '+nt',
          members: [
            member('6', 'op'),
            member('9', 'op'),
            member('1', 'regular'),
            member('5', 'regular'),
          ],
        },
        {
          id: 'libera-offtopic',
          name: 'offtopic',
          type: 'text',
          serverId: 'libera',
          topic: 'Anything goes (within reason)',
          modes: '+nt',
          members: [
            member('6', 'op'),
            member('1', 'regular'),
            member('3', 'regular'),
            member('8', 'regular'),
          ],
        },
      ],
    },
    {
      id: 'oftc',
      name: 'OFTC',
      abbreviation: 'OF',
      channels: [
        {
          id: 'oftc-debian',
          name: 'debian',
          type: 'text',
          serverId: 'oftc',
          topic: 'Debian GNU/Linux support | Current stable: Bookworm 12.9',
          modes: '+Cnt',
          members: [
            member('6', 'op'),
            member('4', 'op'),
            member('1', 'regular'),
            member('10', 'regular'),
          ],
        },
        {
          id: 'oftc-spi',
          name: 'spi',
          type: 'text',
          serverId: 'oftc',
          topic: 'Software in the Public Interest discussion',
          modes: '+nt',
          unread: true,
          members: [
            member('6', 'op'),
            member('4', 'voice'),
            member('1', 'regular'),
          ],
        },
      ],
    },
    {
      id: 'efnet',
      name: 'EFNet',
      abbreviation: 'EF',
      channels: [
        {
          id: 'efnet-irc',
          name: 'irc',
          type: 'text',
          serverId: 'efnet',
          topic: 'IRC discussion | The OG network since 1990',
          modes: '+nt',
          members: [
            member('1', 'op'),
            member('8', 'voice'),
            member('3', 'regular'),
          ],
        },
        {
          id: 'efnet-bots',
          name: 'bots',
          type: 'text',
          serverId: 'efnet',
          topic: 'Bot development and discussion | eggdrop, limnoria, sopel',
          modes: '+nt',
          members: [
            member('8', 'op'),
            member('1', 'regular'),
          ],
        },
      ],
    },
  ]
}
