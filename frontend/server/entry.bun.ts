// Production server entry: Nitro's Bun entry with three changes.
// - The peer address of each request reaches the proxy middleware (server/middleware/backend.ts),
//   which forwards it to the backend. Bun requests have no socket, so h3 can't read one.
// - The idle timeout outlasts the backend's slowest calls; Bun's default closes a request
//   that sends nothing for 10 seconds, and link previews may take up to 75.
// - Request bodies have a size cap, since they are read into memory before routing.
import '#nitro-internal-pollyfills'
import { useNitroApp } from 'nitropack/runtime'
import { startScheduleRunner } from 'nitropack/runtime/internal'
import wsAdapter from 'crossws/adapters/bun'

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
    const body = req.body ? await req.arrayBuffer() : undefined
    return nitroApp.localFetch(url.pathname + url.search, {
      host: url.hostname,
      protocol: url.protocol,
      headers: req.headers,
      method: req.method,
      redirect: req.redirect,
      body,
      // Becomes the request's `__unenv__` context; see peerAddress in server/utils/backend.ts.
      context: { peerAddress: bunServer.requestIP(req)?.address },
    } as RequestInit)
  },
})

console.log(`Listening on ${server.url}...`)
if (import.meta._tasks) startScheduleRunner()
