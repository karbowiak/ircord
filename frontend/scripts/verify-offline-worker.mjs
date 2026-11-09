// Exercise the production worker and real IndexedDB, with a disposable archive.
export async function verifyOfflineWorker(page, url, state) {
  await page.evaluate(
    async ({ url, state }) => {
      const owner = `worker-check-${crypto.randomUUID()}`
      const worker = new Worker(url, { type: 'module' })
      let serial = 0
      const pending = new Map()
      worker.onmessage = ({ data }) => {
        const result = pending.get(data.id)
        pending.delete(data.id)
        if (data.error) result.reject(new Error(data.error))
        else result.resolve(data.result)
      }
      const rpc = (method, ...args) =>
        new Promise((resolve, reject) => {
          const id = ++serial
          pending.set(id, { resolve, reject })
          worker.postMessage({ id, method, args })
        })
      const check = (value, message) => {
        if (!value) throw new Error(message)
      }
      const rejects = async (operation) => {
        let rejected = false
        try {
          await operation()
        } catch {
          rejected = true
        }
        check(rejected, 'Expected worker operation to reject')
      }
      const conversation = state.conversations[0]
      state = { ...state, cursor: 0, conversations: [conversation] }
      const message = {
        id: '原本',
        conversationId: conversation.id,
        senderId: state.user.id,
        kind: 'message',
        text: 'Original 東京',
        createdAt: new Date().toISOString(),
        attachment: {
          url: 'https://example.com/photo.png',
          name: 'photo.png',
          contentType: 'image/png',
        },
      }
      try {
        await rpc('open', owner)
        await rpc('begin', state, new Date().toISOString(), 'first')
        await rpc('page', [message], 0, 'first')
        await rpc('events', [
          { seq: 1, type: 'message.updated', data: { ...message, text: 'Edited 東京' } },
        ])
        await rpc('page', [message], 0, 'first')
        check(
          (await rpc('history', conversation.id, undefined, undefined, 40)).items[0].text ===
            'Edited 東京',
          'Stale snapshot replaced a newer edit',
        )
        await rejects(() => rpc('events', [{ seq: 3, type: 'profile.updated', data: state.user }]))
        check((await rpc('status')).meta.state.cursor === 1, 'Gap advanced the stored cursor')
        const second = { ...message, id: '写真', text: 'Second 東京' }
        await rpc('events', [{ seq: 2, type: 'message.created', data: second }])
        await rpc('saveFile', message.attachment.url, new Blob(['photo'], { type: 'image/png' }))
        await rpc('pin', message.id, true)
        const history = await rpc('history', conversation.id, undefined, undefined, 1)
        check(
          (await rpc('history', conversation.id, history.nextCursor, undefined, 1)).items.length ===
            1,
          'Unicode history cursor failed',
        )
        const many = Array.from({ length: 60 }, (_, i) => ({
          ...message,
          id: `検索-${i}`,
          text: '検索 東京',
          attachment: undefined,
        }))
        await rpc('page', many, 2, 'first')
        const filters = { from: [], mentions: [], in: [], has: [], before: '', after: '' }
        const result = await rpc('search', '東京', filters, undefined, 50)
        check(result.items.length === 50 && result.nextCursor, 'Unicode search first page failed')
        check(
          (await rpc('search', '東京', filters, result.nextCursor, 50)).items.length === 12,
          'Unicode search pagination failed',
        )
        const encodeServer = (value) =>
          btoa(String.fromCharCode(...new TextEncoder().encode(JSON.stringify(value))))
            .replace(/\+/g, '-')
            .replace(/\//g, '_')
            .replace(/=+$/, '')
        const boundary = result.items.at(-1).message
        const serverCursor = encodeServer({
          v: 1,
          at: Date.parse(boundary.createdAt) * 1e6,
          id: boundary.id,
        })
        check(
          (await rpc('search', '東京', filters, serverCursor, 50)).items.length === 12,
          'Server search cursor did not continue locally',
        )
        const onlineHistory = encodeServer({
          at: Date.parse(history.items[0].createdAt) * 1e6,
          id: history.items[0].id,
        })
        check(
          (await rpc('history', conversation.id, onlineHistory, undefined, 40)).items.length > 0,
          'Server history cursor did not continue locally',
        )
        const newerState = (await rpc('status')).meta.state
        await rpc('begin', newerState, new Date().toISOString(), 'replacement')
        await rejects(() => rpc('page', [message], 0, 'first'))
        await rpc('page', [second], 2, 'replacement')
        await rpc('finish', 2, 'replacement')
        check(
          (await rpc('status')).messages === 2,
          'Replacement lost a pin or retained unseen history',
        )
        await rpc('events', [
          {
            seq: 3,
            type: 'message.updated',
            data: { ...second, text: 'Deleted', attachment: undefined, deleted: true },
          },
        ])
        const file = await rpc('file', message.attachment.url)
        check(
          file.blob.size === 5 && file.refs.length === 1 && file.refs[0].id === message.id,
          'Deletion removed a shared pinned file',
        )
        await rpc('events', [
          { seq: 4, type: 'conversation.removed', data: { id: conversation.id } },
        ])
        const final = await rpc('status')
        check(
          final.messages === 0 && final.files === 0,
          'Conversation removal retained messages or pinned files',
        )
      } finally {
        await rpc('clear')
        worker.terminate()
      }
    },
    { url, state },
  )
}
