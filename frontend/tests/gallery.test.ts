import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import type { Message } from '../shared/types/chat'
import { galleryLoads, messageImages } from '../shared/utils/gallery'
import { createSession, listGallery, redactMessage, sendMessage } from '../server/utils/demo-store'

test('the demo catalogues the same images as the backend', () => {
  const cases = JSON.parse(
    readFileSync(new URL('./fixtures/gallery-parity.json', import.meta.url), 'utf8'),
  ) as { name: string; message: Message; images: unknown[] }[]
  assert.ok(cases.length > 0)
  for (const c of cases)
    assert.deepEqual(messageImages(c.message, 'https://chat.example'), c.images, c.name)
})

test('a conversation gallery pages newest first and follows deletions', () => {
  const session = createSession()
  const before = listGallery(session, 'general').total
  const first = sendMessage(session, {
    clientId: 'gallery-1',
    conversationId: 'general',
    text: 'two https://i.imgur.com/g1.png https://i.imgur.com/g2.gif',
  })
  sendMessage(session, {
    clientId: 'gallery-2',
    conversationId: 'general',
    text: 'newer https://i.imgur.com/g3.jpg',
  })
  const page = listGallery(session, 'general', '', 2)
  assert.equal(page.total, before + 3)
  assert.deepEqual(
    page.items.map((i) => i.url),
    ['https://i.imgur.com/g3.jpg', 'https://i.imgur.com/g1.png'],
  )
  assert.equal(page.items[0]?.excerpt, 'newer https://i.imgur.com/g3.jpg')
  assert.ok(page.nextCursor)
  const next = listGallery(session, 'general', page.nextCursor!, 2)
  assert.equal(next.items[0]?.url, 'https://i.imgur.com/g2.gif')
  assert.equal(next.items[0]?.kind, 'gif')
  redactMessage(session, 'general', first.id)
  assert.equal(listGallery(session, 'general').total, before + 1)
  assert.throws(() => listGallery(session, 'general', 'forged'), /invalid cursor/)
  assert.throws(() => listGallery(session, 'missing'), /not found/i)
  // Other sessions never see it.
  assert.equal(listGallery(createSession(), 'general').total, before)
})

test('gallery images load only where the timeline would load them', () => {
  const origin = 'https://chat.example'
  const link = (url: string) => ({ url, source: 'link' as const })
  assert.equal(galleryLoads(link('https://i.imgur.com/a.png'), 'trusted', origin), true)
  assert.equal(galleryLoads(link('https://random.example/a.png'), 'trusted', origin), false)
  assert.equal(
    galleryLoads(link('https://random.example/a.png'), 'trusted', origin, ['random.example']),
    true,
  )
  assert.equal(galleryLoads(link('https://random.example/a.png'), 'all', origin), true)
  assert.equal(galleryLoads(link('https://i.imgur.com/a.png'), 'off', origin), false)
  const preview = { url: 'https://cdn.example/album.jpg', source: 'preview' as const }
  assert.equal(galleryLoads(preview, 'trusted', origin), true)
  assert.equal(galleryLoads(preview, 'off', origin), false)
  const upload = { url: `${origin}/files/a/b/c.png`, source: 'upload' as const }
  assert.equal(galleryLoads(upload, 'off', origin), true)
})
