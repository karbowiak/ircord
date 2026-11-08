/**
 * Hosts Klipy (the GIF picker's provider) serves media from. Keep in step with MEDIA_HOSTS in
 * public/sw.js and the Klipy URL check in backend/internal/irc/actions.go, which can't import this.
 */
export const klipyMediaHosts = ['static.klipy.com', 'media.klipy.com', 'cdn.klipy.com'] as const

/**
 * GIF hosts the app may read (fetch) as well as display, so a hovered GIF can be decoded and
 * paused on the frame it reached. The non-Klipy hosts send CORS headers. GIFs from other hosts
 * still play, but go back to their first frame when they stop. Kept short on purpose: it widens
 * the page's connect-src, which otherwise allows only IRCord's own origins and the Klipy API.
 */
export const gifFetchHosts = [
  ...klipyMediaHosts.map((host) => `https://${host}`),
  'https://media.giphy.com',
  'https://i.giphy.com',
  'https://media.tenor.com',
  'https://i.imgur.com',
]
