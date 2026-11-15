import { localSpeechShardUrl } from '../../../shared/local-speech'

/**
 * Read-aloud's voice model on this device (#204). The engine and its small files are static files
 * under public/speech/, served before this route. Everything else under /speech/ lands here: a
 * weight shard named by the shipped manifests is redirected to Hugging Face at the pinned
 * revision; any other path is a 404, which the engine reads as "this export has no such model".
 * This is upstream's own deployment pattern, and the one place to change the shards' source.
 */
export default defineEventHandler((event) => {
  setResponseHeader(event, 'Cache-Control', 'no-store')
  const target =
    useRuntimeConfig(event).public.localSpeech !== false && ['GET', 'HEAD'].includes(event.method)
      ? localSpeechShardUrl(getRouterParam(event, 'path') ?? '')
      : ''
  if (!target) {
    setResponseStatus(event, 404)
    return { detail: 'Not Found' }
  }
  // The Hub's CDN refused some referrers; the browser sends none to it.
  setResponseHeader(event, 'Referrer-Policy', 'no-referrer')
  return sendRedirect(event, target, 302)
})
