import assert from 'node:assert/strict'
import test from 'node:test'
import { emptyFilters } from '../../../shared/utils/search-filter'
import {
  effectiveSearchSource,
  isSearchResultJumpable,
  loadSearchPage,
  searchCacheKey,
  searchCoverageKey,
  searchRequestParams,
  searchResultAuthor,
  type SearchSourceInfo,
} from './search-source'

test('source resets to retained outside one online conversation and separates cache keys', () => {
  assert.equal(effectiveSearchSource('upstream', 1, false), 'upstream')
  assert.equal(effectiveSearchSource('upstream', 0, false), 'retained')
  assert.equal(effectiveSearchSource('upstream', 2, false), 'retained')
  assert.equal(effectiveSearchSource('upstream', 1, true), 'retained')
  assert.notDeepEqual(
    searchCacheKey('hello', emptyFilters(), false, 'retained'),
    searchCacheKey('hello', emptyFilters(), false, 'upstream'),
  )
  assert.equal(
    searchRequestParams(' hello ', emptyFilters(), 'next', 'upstream').source,
    'upstream',
  )
  assert.equal(
    searchRequestParams('hello', emptyFilters(), undefined, 'retained').source,
    'retained',
  )
})

test('coverage uses reported source and labels a retained fallback', () => {
  assert.equal(
    searchCoverageKey({
      source: {
        requested: 'upstream',
        actual: 'retained',
        coverage: 'retained-indexed',
        completeness: 'unknown',
        reason: 'upstream-unavailable',
      },
    }),
    'collectionsUi.search.fallbackUpstreamUnavailable',
  )
  assert.equal(
    searchCoverageKey({
      source: {
        requested: 'upstream',
        actual: 'upstream',
        coverage: 'upstream-bounded',
        completeness: 'unknown',
      },
    }),
    'collectionsUi.search.coverageUpstream',
  )
})

test('unknown and inherited fallback reasons use the generic retained label', () => {
  for (const reason of ['future-reason', 'constructor', 'toString', '__proto__']) {
    const source = {
      requested: 'upstream',
      actual: 'retained',
      coverage: 'retained-indexed',
      completeness: 'unknown',
      reason: reason as unknown as SearchSourceInfo['reason'],
    } satisfies SearchSourceInfo

    assert.equal(
      searchCoverageKey({ source }),
      'collectionsUi.search.coverageRetained',
      `${reason} must not resolve through the fallback lookup prototype`,
    )
  }
})

test('missing old-server metadata stays generic and offline pages make no coverage claim', () => {
  assert.equal(searchCoverageKey({}), undefined)
  assert.equal(searchCoverageKey({ offline: true }), undefined)
  assert.equal(isSearchResultJumpable({}), true)
})

test('offline search and offline continuations use archive without network requests', async () => {
  let requests = 0
  let archiveReads = 0
  const load = (options: Partial<Parameters<typeof loadSearchPage>[0]> = {}) =>
    loadSearchPage({
      offline: false,
      archiveAvailable: true,
      archive: async () => {
        archiveReads++
        return 'offline'
      },
      server: async () => {
        requests++
        return 'server'
      },
      isUnavailable: () => true,
      ...options,
    })

  assert.equal(await load({ offline: true }), 'offline')
  assert.equal(await load({ cursor: 'offline-search:next' }), 'offline')
  assert.equal(requests, 0)
  assert.equal(archiveReads, 2)
})

test('cancellation and authorization errors do not become offline results', async () => {
  let archiveReads = 0
  const controller = new AbortController()
  controller.abort(new Error('request canceled'))
  await assert.rejects(
    loadSearchPage({
      offline: true,
      archiveAvailable: true,
      signal: controller.signal,
      archive: async () => {
        archiveReads++
        return 'offline'
      },
      server: async () => 'server',
      isUnavailable: () => true,
    }),
    /request canceled/,
  )
  await assert.rejects(
    loadSearchPage({
      offline: false,
      archiveAvailable: true,
      archive: async () => {
        archiveReads++
        return 'offline'
      },
      server: async () => {
        throw Object.assign(new Error('unauthorized'), { status: 401 })
      },
      isUnavailable: (error) => (error as { status?: number }).status !== 401,
    }),
    /unauthorized/,
  )
  assert.equal(archiveReads, 0)
})

test('ephemeral hits cannot open retained history and use only the supplied nickname', () => {
  const result = { context: { available: false, identity: 'ephemeral' }, senderNick: 'Mira' }
  assert.equal(isSearchResultJumpable(result), false)
  assert.equal(searchResultAuthor(result), 'Mira')
  assert.equal(
    searchResultAuthor({ context: { available: false, identity: 'ephemeral' } }),
    undefined,
  )
  assert.equal(isSearchResultJumpable({ context: { available: true, identity: 'retained' } }), true)
})
