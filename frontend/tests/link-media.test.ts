import { describe, expect, test } from 'bun:test'
import { messageMediaLinks, widgetSource, widgetHeight, providerWidgetHeight, widgetPlayerFailed, attachmentMediaAsset, nativePostText, nativeStageRatio } from '../shared/utils/link-media'

describe('message link rendering boundaries', () => {
  test('upload previews use MIME or filename while leaving documents as downloads', () => {
    const file = { url: '/files/opaque-key', name: 'clip.mov', contentType: 'application/octet-stream', width: 320, height: 200, alt: '' }
    expect(attachmentMediaAsset(file, 'https://chat.ircord.com')?.kind).toBe('video')
    expect(attachmentMediaAsset({ ...file, name: 'animation.gif' }, 'https://chat.ircord.com')?.contentType).toBe('image/gif')
    expect(attachmentMediaAsset({ ...file, contentType: 'image/png' }, 'https://chat.ircord.com')?.url).toBe('https://chat.ircord.com/files/opaque-key')
    expect(attachmentMediaAsset({ ...file, name: 'notes.pdf', contentType: 'application/pdf' }, 'https://chat.ircord.com')).toBeNull()
    expect(attachmentMediaAsset({ ...file, url: 'javascript:alert(1)' }, 'https://chat.ircord.com')).toBeNull()
  })
  test('social widget resizing is bound to the active provider and post', () => {
    const source = {} as Window
    const twitter = 'https://platform.twitter.com/embed/Tweet.html?id=123&dnt=true'
    const event = { source, origin: 'https://platform.twitter.com', data: { 'twttr.embed': { method: 'twttr.private.resize', params: [{ height: 666, data: { tweet_id: '123' } }] } } }
    expect(providerWidgetHeight(event, source, twitter)).toBe(666)
    expect(providerWidgetHeight({ ...event, source: {} as Window }, source, twitter)).toBeNull()
    expect(providerWidgetHeight({ ...event, origin: 'null' }, source, twitter)).toBeNull()
    expect(providerWidgetHeight(event, source, twitter.replace('123', '456'))).toBeNull()
    const bluesky = 'https://embed.bsky.app/embed/did:plc:abc/app.bsky.feed.post/abc?id=ircord'
    const resize = { source, origin: 'https://embed.bsky.app', data: { id: 'ircord', height: 780 } }
    expect(providerWidgetHeight(resize, source, bluesky)).toBe(780)
    expect(providerWidgetHeight({ ...resize, data: { id: 'another', height: 780 } }, source, bluesky)).toBeNull()
    expect(providerWidgetHeight({ ...resize, data: { id: 'ircord', height: 100000 } }, source, bluesky)).toBeNull()
  })
  test('player failures come only from the matching TikTok frame',()=>{
    const source={} as Window
    const event={source,origin:'https://www.tiktok.com',data:{'x-tiktok-player':true,type:'onPlayerError',value:{errorCode:1004}}}
    const url='https://www.tiktok.com/player/v1/123'
    expect(widgetPlayerFailed(event,source,url)).toBe(true)
    expect(widgetPlayerFailed({...event,source:{} as Window},source,url)).toBe(false)
    expect(widgetPlayerFailed({...event,origin:'https://www.tiktok.com.evil.test'},source,url)).toBe(false)
    expect(widgetPlayerFailed({...event,data:{...event.data,type:'onStateChange'}},source,url)).toBe(false)
  })
  test('prose links only, with duplicates and punctuation handled', () => {
    expect(messageMediaLinks('See (https://reddit.com/comments/abc). https://reddit.com/comments/abc\n```txt\nhttps://example.com/code\n```\n\x11https://example.com/inline\x11')).toEqual(['https://reddit.com/comments/abc'])
    expect(messageMediaLinks('https://example.com/a_(b) and https://example.com/photo.jpg')).toEqual(['https://example.com/a_(b)', 'https://example.com/photo.jpg'])
  })
  test('widget sources cannot turn into same-origin script frames', () => {
    const token = 'a'.repeat(48)
    const widget = { url: `/api/v1/link-media/widgets/${token}`, external: false, token, height: 350 }
    expect(widgetSource(widget, '/api/v1', 'https://chat.ircord.com')).toBe(`https://chat.ircord.com${widget.url}`)
    expect(widgetSource({ ...widget, external: true, url: 'https://open.spotify.com/embed/track/abc' }, '/api/v1', 'https://chat.ircord.com')).toContain('open.spotify.com')
    for (const url of ['javascript:alert(1)', 'https://chat.ircord.com/api/v1/bootstrap', '/api/v1/bootstrap'])
      expect(widgetSource({ ...widget, external: true, url }, '/api/v1', 'https://chat.ircord.com')).toBe('')
    expect(widgetSource({ ...widget, url: '/api/v1/bootstrap' }, '/api/v1', 'https://chat.ircord.com')).toBe('')
  })
  test('height messages require this opaque frame and exact token/payload', () => {
    const source = {} as Window
    const event = { source, origin: 'null', data: { type: 'ircord:widget-size', token: 'test', height: 352 } }
    expect(widgetHeight(event, source, 'test')).toBe(352)
    expect(widgetHeight({ ...event, source: {} as Window }, source, 'test')).toBeNull()
    expect(widgetHeight({ ...event, origin: 'https://publisher.example' }, source, 'test')).toBeNull()
    expect(widgetHeight(event, source, 'stale')).toBeNull()
    expect(widgetHeight({ ...event, data: { ...event.data, height: 100000 } }, source, 'test')).toBeNull()
    expect(widgetHeight({ ...event, data: { ...event.data, extra: true } }, source, 'test')).toBeNull()
  })
})

describe('native post cards', () => {
  test('bylines drop generic titles and keep the post’s own words', () => {
    expect(nativePostText({ url: 'https://www.instagram.com/p/X/', provider: 'instagram', title: 'Post by sa_sa_ne', author: '楽音', description: '🦋\n\n\n\n#pr' }))
      .toEqual({ title: '', caption: '🦋\n\n#pr', name: '楽音', handle: '@sa_sa_ne' })
    expect(nativePostText({ url: 'https://www.reddit.com/r/habin_s2_/comments/1/habin/', provider: 'reddit', title: 'Habin', author: 'InsideBag3727' }))
      .toEqual({ title: 'Habin', caption: '', name: 'u/InsideBag3727', handle: 'r/habin_s2_' })
    expect(nativePostText({ url: 'https://www.tiktok.com/@someone/video/1', provider: 'tiktok', title: 'A long caption that got cut...', author: 'someone', description: 'A long caption that got cut short by the extractor' }))
      .toEqual({ title: '', caption: 'A long caption that got cut short by the extractor', name: '', handle: '@someone' })
  })
  test('gallery stages size for their tallest item within sane bounds', () => {
    expect(nativeStageRatio([{ width: 1440, height: 1920 }, { width: 1200, height: 800 }])).toBe(0.75)
    expect(nativeStageRatio([{ width: 100, height: 1000 }])).toBe(9 / 16)
    expect(nativeStageRatio([{}], 4 / 5)).toBe(0.8)
  })
})
