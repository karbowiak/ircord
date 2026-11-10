import assert from 'node:assert/strict'
import { computed, effectScope, ref, shallowRef } from 'vue'
import { useCursorPages } from '../app/composables/shared/useCursorPages.ts'

const item = (id, text = id) => ({ id, text })
function fixture({ duplicates, resetFirst = false } = {}) {
  const scope = effectScope()
  const first = shallowRef({ items: [item('first')], nextCursor: 'page-2' })
  const filter = ref('all')
  const revision = ref(0)
  const enabled = ref(true)
  const requests = []
  const pages = scope.run(() =>
    useCursorPages({
      firstPage: first,
      resetOn: resetFirst ? [filter, revision, first] : [filter, revision],
      itemKey: (value) => value.id,
      enabled,
      duplicates,
      load: (cursor, signal) =>
        new Promise((resolve, reject) => {
          requests.push({ cursor, signal, filter: filter.value, resolve, reject })
        }),
    }),
  )
  return { scope, first, filter, revision, enabled, requests, ...pages }
}

// Empty and duplicate-only pages must retain their own cursor, including a terminal one.
for (const items of [[], [item('first', 'older duplicate')]]) {
  const pages = fixture()
  const read = pages.loadMore()
  pages.requests[0].resolve({ items, nextCursor: null })
  await read
  assert.deepEqual(pages.items.value, [item('first')])
  assert.equal(pages.cursor.value, null)
  assert.equal(pages.older.value.length, 0)
  await pages.loadMore()
  assert.equal(pages.requests.length, 1, 'a terminal page cannot repeat the first cursor')
  pages.scope.stop()
}
{
  const pages = fixture()
  const empty = pages.loadMore()
  pages.requests[0].resolve({ items: [], nextCursor: 'page-3' })
  await empty
  assert.equal(pages.cursor.value, 'page-3', 'an empty nonterminal page still advances')
  const final = pages.loadMore()
  assert.equal(pages.requests[1].cursor, 'page-3')
  pages.requests[1].resolve({ items: [] })
  await final
  assert.equal(pages.cursor.value, null, 'an omitted next cursor also records the end')
  pages.scope.stop()
}

// Collections retain the first item; search keeps later versions in the same position.
for (const duplicates of ['keep-first', 'keep-last']) {
  const pages = fixture({ duplicates })
  const read = pages.loadMore()
  await pages.loadMore()
  assert.equal(pages.requests.length, 1, 'one generation has at most one pending read')
  pages.requests[0].resolve({
    items: [
      item('first', 'later first'),
      item('second', 'old second'),
      item('second', 'new second'),
    ],
    nextCursor: 'page-3',
  })
  await read
  assert.deepEqual(
    pages.items.value,
    duplicates === 'keep-first'
      ? [item('first'), item('second', 'old second')]
      : [item('first', 'later first'), item('second', 'new second')],
  )
  const next = pages.loadMore()
  pages.requests[1].resolve({
    items: [item('second', 'last second'), item('third')],
    nextCursor: null,
  })
  await next
  assert.deepEqual(
    pages.items.value.map((value) => value.id),
    ['first', 'second', 'third'],
  )
  assert.equal(
    pages.items.value[1].text,
    duplicates === 'keep-first' ? 'old second' : 'last second',
  )
  pages.scope.stop()
}

// A filter/revision reset cancels old reads without letting their outcomes affect replacements.
for (const outcome of ['resolve', 'reject']) {
  const pages = fixture()
  const old = pages.loadMore()
  pages.filter.value = 'mentions'
  assert(pages.requests[0].signal.aborted)
  assert.equal(pages.loadingMore.value, false)
  assert.equal(pages.cursor.value, 'page-2')
  const current = pages.loadMore()
  assert.equal(pages.requests[1].filter, 'mentions')
  if (outcome === 'resolve')
    pages.requests[0].resolve({ items: [item('obsolete')], nextCursor: null })
  else pages.requests[0].reject(new Error('obsolete failure'))
  await old
  assert.equal(pages.loadingMore.value, true, 'old completion cannot release a replacement')
  assert.equal(pages.moreError.value, '')
  assert.deepEqual(pages.items.value, [item('first')])
  pages.requests[1].resolve({ items: [item('current')], nextCursor: 'page-3' })
  await current
  assert.equal(pages.loadingMore.value, false)
  assert.equal(pages.cursor.value, 'page-3')
  assert.deepEqual(pages.items.value, [item('first'), item('current')])

  const pending = pages.loadMore()
  pages.revision.value++
  assert(pages.requests[2].signal.aborted)
  pages.requests[2].reject(new Error('old revision failure'))
  await pending
  assert.equal(pages.moreError.value, '')
  assert.deepEqual(pages.older.value, [])
  pages.scope.stop()
}
{
  const pages = fixture()
  const old = pages.loadMore()
  pages.filter.value = 'other'
  pages.filter.value = 'all'
  pages.requests[0].resolve({ items: [item('obsolete round trip')], nextCursor: null })
  await old
  assert.deepEqual(
    pages.older.value,
    [],
    'returning to the same filter does not revive an old read',
  )
  pages.scope.stop()
}

// Current failures are visible and retry the same cursor; a successful retry clears the error.
{
  const pages = fixture()
  const failure = new Error('current failure')
  const failed = pages.loadMore()
  pages.requests[0].reject(failure)
  await failed
  assert.equal(pages.moreError.value, failure)
  assert.equal(pages.loadingMore.value, false)
  assert.equal(pages.cursor.value, 'page-2')
  const retry = pages.loadMore()
  assert.equal(pages.moreError.value, '')
  assert.equal(pages.requests[1].cursor, 'page-2')
  pages.requests[1].resolve({ items: [item('retry')], nextCursor: null })
  await retry
  assert.deepEqual(pages.items.value, [item('first'), item('retry')])
  pages.scope.stop()
}

// Saved patches keep older results and their cursor; removal remains a domain filter.
{
  const pages = fixture()
  const read = pages.loadMore()
  pages.requests[0].resolve({ items: [item('saved')], nextCursor: 'page-3' })
  await read
  pages.first.value = { items: [item('first', 'edited first')], nextCursor: 'new-first-cursor' }
  pages.older.value = pages.older.value.map((value) => ({ ...value, text: 'edited saved' }))
  assert.deepEqual(pages.items.value, [
    item('first', 'edited first'),
    item('saved', 'edited saved'),
  ])
  assert.equal(pages.cursor.value, 'page-3', 'source edits preserve the reading position')
  const removed = ref(new Set())
  const visible = computed(() => pages.items.value.filter((value) => !removed.value.has(value.id)))
  removed.value = new Set(['saved'])
  assert.deepEqual(visible.value, [item('first', 'edited first')])
  assert.equal(pages.cursor.value, 'page-3')
  pages.revision.value++
  assert.deepEqual(pages.older.value, [])
  assert.equal(pages.cursor.value, 'new-first-cursor')
  pages.scope.stop()
}

// Search resets on first-page replacement and online/offline identity, and gates disabled reads.
{
  const pages = fixture({ duplicates: 'keep-last', resetFirst: true })
  const old = pages.loadMore()
  pages.first.value = { items: [item('replacement')], nextCursor: 'replacement-cursor' }
  assert(pages.requests[0].signal.aborted)
  pages.requests[0].reject(new Error('replaced first page'))
  await old
  assert.equal(pages.moreError.value, '')
  assert.deepEqual(pages.items.value, [item('replacement')])
  const online = pages.loadMore()
  pages.filter.value = 'offline-search'
  assert(pages.requests[1].signal.aborted)
  pages.requests[1].resolve({ items: [item('online result')], nextCursor: null })
  await online
  assert.deepEqual(pages.older.value, [])
  pages.enabled.value = false
  assert.equal(pages.cursor.value, null)
  assert.deepEqual(pages.items.value, [])
  await pages.loadMore()
  assert.equal(pages.requests.length, 2)
  pages.enabled.value = true
  assert.equal(pages.cursor.value, 'replacement-cursor')
  pages.scope.stop()
}

// Links' explicit refresh and scope disposal also invalidate an in-flight page.
{
  const pages = fixture()
  const old = pages.loadMore()
  pages.reset()
  assert(pages.requests[0].signal.aborted)
  pages.requests[0].resolve({ items: [item('before refresh')], nextCursor: null })
  await old
  assert.deepEqual(pages.older.value, [])
  const pending = pages.loadMore()
  pages.scope.stop()
  assert(pages.requests[1].signal.aborted)
  pages.requests[1].reject(new Error('disposed read'))
  await pending
  await pages.loadMore()
  assert.equal(pages.requests.length, 2)
  assert.equal(pages.loadingMore.value, false)
  assert.equal(pages.moreError.value, '')
}

console.log(
  'Cursor pages: terminal empty/duplicate pages, cursor advance, deduplication policies, generation races, retries, edits, resets and disposal passed.',
)
