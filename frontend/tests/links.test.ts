import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  groupMedia,
  maxMediaLinks,
  maxPlayers,
  mediaLinks,
  youtubeEmbedUrl,
  youtubeVideo,
  youtubeWatchUrl,
} from '../shared/utils/links'

const origin = 'https://chat.example'

test('YouTube URL variants normalize to one video', () => {
  const id = 'dQw4w9WgXcQ'
  for (const href of [
    `https://www.youtube.com/watch?v=${id}`,
    `https://m.youtube.com/watch?feature=share&v=${id}`,
    `https://music.youtube.com/watch?v=${id}`,
    `https://youtu.be/${id}?si=tracking`,
    `https://www.youtube.com/embed/${id}`,
    `https://www.youtube-nocookie.com/embed/${id}`,
    `https://www.youtube.com/live/${id}`,
  ])
    assert.deepEqual(youtubeVideo(href), { id, short: false, start: 0 }, href)
  assert.deepEqual(youtubeVideo('https://www.youtube.com/shorts/abcdefghijk'), {
    id: 'abcdefghijk',
    short: true,
    start: 0,
  })
  assert.equal(youtubeVideo(`https://youtu.be/${id}?t=1m30s`)?.start, 90)
  assert.equal(youtubeVideo(`https://youtube.com/watch?v=${id}&t=42s`)?.start, 42)
  assert.equal(youtubeVideo(`https://youtu.be/${id}?t=99999999`)?.start, 0)
  // At most a day: longer offsets are ignored.
  assert.equal(youtubeVideo(`https://youtu.be/${id}?t=999999`)?.start, 0)
  assert.equal(youtubeVideo(`https://youtu.be/${id}?t=25h`)?.start, 0)
  assert.equal(youtubeVideo(`https://youtu.be/${id}?t=86400`)?.start, 86400)
  // An empty t= falls back to start=, as the backend does.
  assert.equal(youtubeVideo(`https://youtu.be/${id}?t=&start=30`)?.start, 30)
  for (const href of [
    `http://youtu.be/${id}`,
    'https://youtube.com/watch?v=short',
    'https://www.youtube.com/channel/UCabcdefghij',
    `https://youtube.com.evil.test/watch?v=${id}`,
    `https://youtu.be/${id}%22onload%3D`,
    `https://user@youtu.be/${id}`,
  ])
    assert.equal(youtubeVideo(href), null, href)
})

test('the player URL is built from the ID alone, on the privacy-enhanced host', () => {
  const video = { id: 'dQw4w9WgXcQ', short: false, start: 90 }
  const url = new URL(youtubeEmbedUrl(video))
  assert.equal(url.origin, 'https://www.youtube-nocookie.com')
  assert.equal(url.pathname, '/embed/dQw4w9WgXcQ')
  assert.equal(url.searchParams.get('start'), '90')
  assert.equal(url.searchParams.get('autoplay'), '1')
  assert.equal(youtubeWatchUrl(video), 'https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=90s')
  assert.equal(
    youtubeWatchUrl({ id: 'abcdefghijk', short: true, start: 0 }),
    'https://www.youtube.com/shorts/abcdefghijk',
  )
})

test('typed media links: YouTube, audio, video and images, without duplicates', () => {
  const text =
    'https://youtu.be/dQw4w9WgXcQ https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=5 ' +
    'https://i.imgur.com/a.png https://i.imgur.com/a.png https://cdn.discordapp.com/song.mp3 ' +
    'https://cdn.discordapp.com/clip.webm https://example.com/page'
  const links = mediaLinks(text, 'trusted', origin, maxMediaLinks)
  assert.deepEqual(
    links.map((l) => l.kind),
    ['youtube', 'image', 'audio', 'video'],
  )
  assert.equal(links[0]!.youtube?.id, 'dQw4w9WgXcQ')
  assert.equal(mediaLinks(text, 'off', origin).length, 0)
  // Untrusted hosts only under "all"; YouTube is a well-known host.
  assert.deepEqual(
    mediaLinks('https://files.example/a.ogg https://youtu.be/dQw4w9WgXcQ', 'trusted', origin).map(
      (l) => l.kind,
    ),
    ['youtube'],
  )
  assert.equal(mediaLinks('https://files.example/a.ogg', 'all', origin)[0]?.kind, 'audio')
  // GitHub's share-image host is trusted like the other well-known hosts.
  assert.equal(
    mediaLinks('https://opengraph.githubassets.com/1/karbowiak/ircord.png', 'trusted', origin)[0]
      ?.kind,
    'image',
  )
})

test('several images form one gallery and players are bounded per message', () => {
  const images = Array.from({ length: 14 }, (_, i) => `https://i.imgur.com/${i}.jpg`).join(' ')
  assert.equal(mediaLinks(images, 'trusted', origin).length, 3)
  const links = mediaLinks(images, 'trusted', origin, maxMediaLinks)
  assert.equal(links.length, 10)
  assert.equal(groupMedia(links).images.length, 10)
  const videos = Array.from({ length: 5 }, (_, i) => `https://cdn.discordapp.com/${i}.mp4`)
  const grouped = groupMedia(mediaLinks(videos.join(' '), 'trusted', origin, maxMediaLinks))
  assert.equal(grouped.players.length, maxPlayers)
  assert.equal(grouped.images.length, 0)
})
