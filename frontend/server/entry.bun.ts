// Production server entry: Nitro's Bun entry with these changes.
// - The peer address of each request reaches the proxy middleware (server/middleware/backend.ts),
//   which forwards it to the backend. Bun requests have no socket, so h3 can't read one.
// - Backend calls and files stream in both directions. Nitro's in-process fetch collects a
//   whole response before returning it, so the proxy hands the backend's response back here
//   (`upstream`) and sends the incoming request's body on unread.
// - Other routes read the request body only when they ask for it, so a request a handler turns
//   away (signed out, wrong origin) is never held in memory.
// - The idle timeout outlasts the backend's slowest calls; Bun's default closes a request
//   that sends nothing for 10 seconds, and link previews may take up to 75.
// - Request bodies have a size cap.
import '#nitro-internal-pollyfills'
import { useNitroApp } from 'nitropack/runtime'
import { startScheduleRunner } from 'nitropack/runtime/internal'
import wsAdapter from 'crossws/adapters/bun'
import type { EntryContext } from './utils/backend'

interface BunServer {
  url: URL
  requestIP(request: Request): { address: string } | null
}
declare const Bun: {
  serve(options: {
    port: string | number
    hostname?: string
    idleTimeout?: number
    maxRequestBodySize?: number
    websocket?: unknown
    fetch(request: Request, server: BunServer): Response | Promise<Response> | undefined
  }): BunServer
}

/** Seconds; Bun allows at most 255. */
const idleTimeout = 120
/** The largest upload (25 MiB) plus room for its form encoding. */
const maxRequestBodySize = 32 << 20

const nitroApp = useNitroApp()
const ws = import.meta._websocket ? wsAdapter(nitroApp.h3App.websocket) : undefined
const number = (value: string | undefined) => {
  const parsed = Number.parseInt(value ?? '')
  return Number.isNaN(parsed) ? undefined : parsed
}

/**
 * The request body for h3's readBody and friends, read on first use. h3 resolves the request's
 * `body` with Promise.resolve, which calls `then` only when a handler asks for it.
 */
const lazyBody = (req: Request) => {
  let read: Promise<Buffer> | undefined
  return {
    then<T, E = never>(
      resolve: (body: Buffer) => T | PromiseLike<T>,
      reject?: (error: unknown) => E | PromiseLike<E>,
    ) {
      read ??= req.arrayBuffer().then((body) => Buffer.from(body))
      return read.then(resolve, reject)
    },
  }
}

/** Statuses that never carry a response body; neither does an answer to HEAD. */
const bodyless = new Set([101, 204, 205, 304])

/** The backend's response body under the status and headers Nitro settled on. */
function streamed(response: Response, upstream: Response, method: string) {
  if (bodyless.has(response.status) || method === 'HEAD') {
    void upstream.body?.cancel()
    return response
  }
  return new Response(upstream.body, {
    status: response.status,
    statusText: response.statusText,
    headers: response.headers,
  })
}

const server = Bun.serve({
  port: process.env.NITRO_PORT || process.env.PORT || 3000,
  hostname: process.env.NITRO_HOST || process.env.HOST,
  idleTimeout: number(process.env.NITRO_BUN_IDLE_TIMEOUT) ?? idleTimeout,
  maxRequestBodySize,
  websocket: import.meta._websocket ? ws!.websocket : undefined,
  async fetch(req, bunServer) {
    if (import.meta._websocket && req.headers.get('upgrade') === 'websocket')
      return ws!.handleUpgrade(req, bunServer as never)
    const url = new URL(req.url)
    // Becomes the request's `__unenv__` context; see EntryContext in server/utils/backend.ts.
    const context: EntryContext = { peerAddress: bunServer.requestIP(req)?.address, request: req }
    const response = await nitroApp.localFetch(url.pathname + url.search, {
      host: url.hostname,
      protocol: url.protocol,
      headers: req.headers,
      method: req.method,
      redirect: req.redirect,
      body: req.body ? lazyBody(req) : undefined,
      context,
    } as unknown as RequestInit)
    return context.upstream ? streamed(response, context.upstream, req.method) : response
  },
})

console.log(`Listening on ${server.url}...`)
if (import.meta._tasks) startScheduleRunner()
