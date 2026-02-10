import type { Server, Channel, DMChannel, Message, User, ChannelMember } from '~/types'

export function useMockData() {
  const users: User[] = [
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

  const mkMember = (userId: string, mode: 'op' | 'voice' | 'regular'): ChannelMember => ({
    user: users.find(u => u.id === userId)!,
    mode,
  })

  const servers: Server[] = [
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
            mkMember('6', 'op'),
            mkMember('1', 'op'),
            mkMember('2', 'voice'),
            mkMember('5', 'voice'),
            mkMember('3', 'regular'),
            mkMember('10', 'regular'),
            mkMember('8', 'regular'),
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
            mkMember('6', 'op'),
            mkMember('2', 'op'),
            mkMember('1', 'voice'),
            mkMember('4', 'regular'),
            mkMember('9', 'regular'),
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
            mkMember('6', 'op'),
            mkMember('9', 'op'),
            mkMember('1', 'regular'),
            mkMember('5', 'regular'),
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
            mkMember('6', 'op'),
            mkMember('1', 'regular'),
            mkMember('3', 'regular'),
            mkMember('8', 'regular'),
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
            mkMember('6', 'op'),
            mkMember('4', 'op'),
            mkMember('1', 'regular'),
            mkMember('10', 'regular'),
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
            mkMember('6', 'op'),
            mkMember('4', 'voice'),
            mkMember('1', 'regular'),
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
            mkMember('1', 'op'),
            mkMember('8', 'voice'),
            mkMember('3', 'regular'),
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
            mkMember('8', 'op'),
            mkMember('1', 'regular'),
          ],
        },
      ],
    },
  ]

  const dms: DMChannel[] = [
    { id: 'dm1', recipient: users[1], unread: true },
    { id: 'dm2', recipient: users[2] },
    { id: 'dm3', recipient: users[4], unread: true },
  ]

  const messages: Message[] = [
    // #linux on Libera
    { id: 'm1', channelId: 'libera-linux', author: users[1], content: 'anyone running 6.12 yet? any regressions?', timestamp: '14:30' },
    { id: 'm2', channelId: 'libera-linux', author: users[9], content: 'been running it for a week, no issues on my thinkpad', timestamp: '14:31' },
    { id: 'm3', channelId: 'libera-linux', author: users[0], content: 'same here, the new scheduler improvements are noticeable', timestamp: '14:32' },
    { id: 'm4', channelId: 'libera-linux', author: users[7], content: 'i had a weird BTRFS bug but might be unrelated', timestamp: '14:33' },
    { id: 'm5', channelId: 'libera-linux', author: users[1], content: 'evilpete: what kind of bug? snapshot related?', timestamp: '14:34' },
    { id: 'm6', channelId: 'libera-linux', author: users[7], content: 'yeah, scrub was throwing errors on a RAID1 array. turned out to be a bad cable lol', timestamp: '14:35' },

    // #programming on Libera
    { id: 'm7', channelId: 'libera-programming', author: users[8], content: 'is there a good reason to use C over Rust for new projects in 2026?', timestamp: '15:00' },
    { id: 'm8', channelId: 'libera-programming', author: users[1], content: 'embedded systems with limited toolchain support, legacy codebases', timestamp: '15:01' },
    { id: 'm9', channelId: 'libera-programming', author: users[0], content: 'also kernel modules, though Rust support in the kernel is getting better', timestamp: '15:02' },
    { id: 'm10', channelId: 'libera-programming', author: users[3], content: 'the real answer is "it depends" as always :)', timestamp: '15:05' },

    // #rust on Libera
    { id: 'm11', channelId: 'libera-rust', author: users[8], content: 'just published my first crate, feeling good', timestamp: '12:00' },
    { id: 'm12', channelId: 'libera-rust', author: users[0], content: 'nice, what does it do?', timestamp: '12:01' },
    { id: 'm13', channelId: 'libera-rust', author: users[8], content: 'async IRC client library. figured there was room for one more', timestamp: '12:02' },

    // #debian on OFTC
    { id: 'm14', channelId: 'oftc-debian', author: users[9], content: 'is there an ETA for trixie?', timestamp: '10:15' },
    { id: 'm15', channelId: 'oftc-debian', author: users[3], content: 'freeze is underway, probably mid-2026', timestamp: '10:20' },

    // DM with alice
    { id: 'm16', channelId: 'dm1', author: users[1], content: 'hey, did you see the new IRCv3 spec draft?', timestamp: '09:00' },
    { id: 'm17', channelId: 'dm1', author: users[0], content: 'yeah, message tags look interesting. might implement them', timestamp: '09:05' },
    { id: 'm18', channelId: 'dm1', author: users[1], content: 'let me know if you need help testing', timestamp: '09:06' },
  ]

  return { users, servers, dms, messages }
}
