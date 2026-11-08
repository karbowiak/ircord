/**
 * GIF hosts the app may read (fetch) as well as display, so a hovered GIF can be decoded and
 * paused on the frame it reached. Each sends CORS headers. GIFs from other hosts still play,
 * but go back to their first frame when they stop. Kept short on purpose: it widens the page's
 * connect-src, which otherwise allows only IRCord's own origins and the Klipy API.
 */
export const gifFetchHosts = [
  'https://static.klipy.com',
  'https://media.klipy.com',
  'https://cdn.klipy.com',
  'https://media.giphy.com',
  'https://i.giphy.com',
  'https://media.tenor.com',
  'https://i.imgur.com',
] as const
