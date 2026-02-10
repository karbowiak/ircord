import type { Message } from '~/types'
import type { UsersById } from '~/mocks/chat/helpers'

export function buildMockMessages(usersById: UsersById): Message[] {
  return [
    { id: 'm1', channelId: 'libera-linux', author: usersById['2'], content: 'anyone running 6.12 yet? any regressions?', timestamp: '14:30' },
    { id: 'm2', channelId: 'libera-linux', author: usersById['10'], content: 'been running it for a week, no issues on my thinkpad', timestamp: '14:31' },
    { id: 'm3', channelId: 'libera-linux', author: usersById['1'], content: 'same here, the new scheduler improvements are noticeable', timestamp: '14:32' },
    { id: 'm4', channelId: 'libera-linux', author: usersById['8'], content: 'i had a weird BTRFS bug but might be unrelated', timestamp: '14:33' },
    { id: 'm5', channelId: 'libera-linux', author: usersById['2'], content: 'evilpete: what kind of bug? snapshot related?', timestamp: '14:34' },
    { id: 'm6', channelId: 'libera-linux', author: usersById['8'], content: 'yeah, scrub was throwing errors on a RAID1 array. turned out to be a bad cable lol', timestamp: '14:35' },
    { id: 'm7', channelId: 'libera-programming', author: usersById['9'], content: 'is there a good reason to use C over Rust for new projects in 2026?', timestamp: '15:00' },
    { id: 'm8', channelId: 'libera-programming', author: usersById['2'], content: 'embedded systems with limited toolchain support, legacy codebases', timestamp: '15:01' },
    { id: 'm9', channelId: 'libera-programming', author: usersById['1'], content: 'also kernel modules, though Rust support in the kernel is getting better', timestamp: '15:02' },
    { id: 'm10', channelId: 'libera-programming', author: usersById['4'], content: 'the real answer is "it depends" as always :)', timestamp: '15:05' },
    { id: 'm11', channelId: 'libera-rust', author: usersById['9'], content: 'just published my first crate, feeling good', timestamp: '12:00' },
    { id: 'm12', channelId: 'libera-rust', author: usersById['1'], content: 'nice, what does it do?', timestamp: '12:01' },
    { id: 'm13', channelId: 'libera-rust', author: usersById['9'], content: 'async IRC client library. figured there was room for one more', timestamp: '12:02' },
    { id: 'm14', channelId: 'oftc-debian', author: usersById['10'], content: 'is there an ETA for trixie?', timestamp: '10:15' },
    { id: 'm15', channelId: 'oftc-debian', author: usersById['4'], content: 'freeze is underway, probably mid-2026', timestamp: '10:20' },
    { id: 'm16', channelId: 'dm1', author: usersById['2'], content: 'hey, did you see the new IRCv3 spec draft?', timestamp: '09:00' },
    { id: 'm17', channelId: 'dm1', author: usersById['1'], content: 'yeah, message tags look interesting. might implement them', timestamp: '09:05' },
    { id: 'm18', channelId: 'dm1', author: usersById['2'], content: 'let me know if you need help testing', timestamp: '09:06' },
  ]
}
