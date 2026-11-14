import assert from 'node:assert/strict'
import test from 'node:test'
import type { ServerLine } from '../types/chat'
import { groupServerLog } from './server-log'

let next = 0
const line = (kind: string, command: string, text: string, source?: string) =>
  ({ id: String(++next), kind, command, text, source, time: '' }) as unknown as ServerLine

test('echoes of sent commands are dropped and read-marker runs fold into one group', () => {
  const items = groupServerLog([
    line('outgoing', 'MARKREAD', 'timestamp=1'),
    line('raw', 'MARKREAD', 'timestamp=1', 'irc.example'),
    line('outgoing', 'MARKREAD', 'timestamp=2'),
    line('raw', 'MARKREAD', 'timestamp=2', 'irc.example'),
    line('outgoing', 'MARKREAD', 'timestamp=3'),
    line('numeric', '001', 'Welcome'),
  ])
  assert.equal(items.length, 2)
  assert.equal(items[0]!.type === 'group' && items[0]!.lines.length, 3)
  assert.equal(items[1]!.type === 'line' && items[1]!.line.command, '001')
})

test('HistServ joins and quits fold; short runs stay as lines', () => {
  const histserv = (text: string) => line('notice', 'NOTICE', text, 'HistServ!HistServ@localhost')
  const items = groupServerLog([
    histserv('[#general] jay quit (connection closed)'),
    histserv('[#general] jay [account: jay] joined the channel'),
    histserv('[#general] ana quit (connection closed)'),
    line('raw', 'AWAY', 'away'),
    line('numeric', '306', 'You have been marked as being away'),
    histserv('Some other notice'),
  ])
  assert.deepEqual(
    items.map((item) => (item.type === 'group' ? `group:${item.family}` : item.line.command)),
    ['group:presence', 'AWAY', '306', 'NOTICE'],
  )
})
