import type { Message, MessagePage } from '../../shared/types/chat'

// Go emits RFC3339 timestamps with up to nanosecond precision. Date.parse alone
// loses the sub-millisecond part and can select the wrong timeline boundary.
function nanoseconds(value: string): bigint | undefined {
  const match = /^(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(?:\.(\d{1,9}))?(Z|[+-]\d\d:\d\d)$/.exec(value)
  if (!match) return
  const milliseconds = Date.parse(`${match[1]}${match[3]}`)
  if (!Number.isFinite(milliseconds)) return
  return BigInt(milliseconds) * 1_000_000n + BigInt((match[2] ?? '').padEnd(9, '0'))
}

/** Pick an actual displayed, retained message, never a replay or pagination cursor. */
export function observedReadBoundary(
  page: MessagePage | undefined,
  conversation: string,
): string | undefined {
  if (!page || page.partial || page.offline || page.syncError || page.detached || page.hasNewer)
    return
  let best: { message: Message; at: bigint; position: number } | undefined
  for (const message of page.items) {
    if (
      message.conversationId !== conversation ||
      message.delivery ||
      !['message', 'action', 'notice'].includes(message.kind)
    )
      continue
    const at = nanoseconds(message.createdAt)
    const position = message.position ?? 0
    // ASCII IDs compare identically in JavaScript and SQLite's binary collation.
    if (
      at === undefined ||
      !Number.isSafeInteger(position) ||
      !/^[\x21-\x7e]{1,200}$/.test(message.id)
    )
      return
    if (
      !best ||
      at > best.at ||
      (at === best.at &&
        (position > best.position || (position === best.position && message.id > best.message.id)))
    )
      best = { message, at, position }
  }
  return best?.message.id
}

/** One bounded OpenAPI discovery per socket session; failures remain fail-closed. */
export function createReadThroughDiscovery(apiBase: string, fetcher: typeof fetch = fetch) {
  let generation = 0
  let pending: Promise<boolean> | undefined
  let controller: AbortController | undefined
  return {
    reset() {
      generation++
      controller?.abort()
      controller = undefined
      pending = undefined
    },
    supported(): Promise<boolean> {
      if (pending) return pending
      const current = generation
      const request = new AbortController()
      controller = request
      pending = (async () => {
        const timeout = setTimeout(() => request.abort(), 5000)
        try {
          const url = new URL(apiBase.replace(/\/api\/v1\/?$/, '') + '/openapi.json', location.href)
          const response = await fetcher(url, {
            credentials: 'include',
            cache: 'no-store',
            signal: request.signal,
          })
          if (!response.ok || !response.body) return false
          const reader = response.body.getReader()
          const chunks: Uint8Array[] = []
          let size = 0
          for (;;) {
            const { done, value } = await reader.read()
            if (done) break
            size += value.byteLength
            if (size > 2_000_000) {
              await reader.cancel()
              return false
            }
            chunks.push(value)
          }
          const bytes = new Uint8Array(size)
          let offset = 0
          for (const chunk of chunks) {
            bytes.set(chunk, offset)
            offset += chunk.byteLength
          }
          const doc = JSON.parse(new TextDecoder().decode(bytes))
          const schema =
            doc?.paths?.['/api/v1/conversations/{id}']?.patch?.requestBody?.content?.[
              'application/json'
            ]?.schema
          const ref = schema?.$ref
          const name =
            typeof ref === 'string' && ref.startsWith('#/components/schemas/')
              ? ref.slice('#/components/schemas/'.length)
              : ''
          const body = name ? doc?.components?.schemas?.[name] : schema
          return current === generation && body?.properties?.readThroughMessageId?.type === 'string'
        } catch {
          return false
        } finally {
          clearTimeout(timeout)
          if (controller === request) controller = undefined
        }
      })()
      return pending
    },
  }
}
