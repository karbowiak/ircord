import { test } from 'node:test'
import assert from 'node:assert/strict'
import { capabilityRows } from '../shared/utils/connection'
import { demoConnectionInfo } from '../server/utils/connection-mock'
import { createFixtures } from '../server/utils/fixtures'
import { parseServerEvent } from '../app/utils/transport'

test('capability details distinguish advertised values from negotiated support', () => {
  const rows = capabilityRows({
    offeredCapabilities: {
      sasl: 'PLAIN,EXTERNAL',
      'draft/multiline': 'max-bytes=4000,max-lines=20',
    },
    negotiatedCapabilities: ['draft/multiline', 'server-time'],
  })
  assert.deepEqual(
    rows.map((r) => [r.name, r.active, r.value]),
    [
      ['draft/multiline', true, 'max-bytes=4000,max-lines=20'],
      ['sasl', false, 'PLAIN,EXTERNAL'],
      ['server-time', true, ''],
    ],
  )
  assert.equal(capabilityRows({ offeredCapabilities: {}, negotiatedCapabilities: [] }).length, 0)
})

test('the demo handshake follows each network and the generated live-event contract', () => {
  const { bootstrap } = createFixtures()
  const rich = demoConnectionInfo(bootstrap.networks[0]!, 1000)
  const limited = demoConnectionInfo(
    bootstrap.networks.find((n) => n.id === 'oftc')!,
    1000,
  )
  assert.ok(rich.negotiatedCapabilities.includes('draft/multiline'))
  assert.ok(!limited.negotiatedCapabilities.includes('draft/multiline'))
  assert.equal(rich.offeredCapabilities.sasl, 'PLAIN,EXTERNAL')
  assert.ok(!rich.negotiatedCapabilities.includes('sasl'))
  assert.equal(
    parseServerEvent(JSON.stringify({ v: 1, seq: 1, type: 'connection.updated', data: rich })).type,
    'connection.updated',
  )
})
