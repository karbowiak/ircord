import { describe, expect, test } from 'bun:test'
import {
  collapsedItems,
  maxCollapsedSections,
  sanitizeCollapsed,
  sectionKey,
  toggleCollapsed,
} from '../shared/utils/sidebar'

describe('collapsible conversation sections', () => {
  test('keys are per network and section', () => {
    expect(sectionKey('libera', 'channels')).toBe('libera/channels')
    expect(sectionKey('oftc', 'channels')).not.toBe(sectionKey('libera', 'channels'))
  })

  test('toggling collapses and expands', () => {
    const collapsed = toggleCollapsed([], 'libera/channels')
    expect(collapsed).toEqual(['libera/channels'])
    expect(toggleCollapsed(collapsed, 'libera/directs')).toEqual([
      'libera/channels',
      'libera/directs',
    ])
    expect(toggleCollapsed(collapsed, 'libera/channels')).toEqual([])
  })

  test('stored values are cleaned and bounded', () => {
    expect(sanitizeCollapsed(undefined)).toEqual([])
    expect(sanitizeCollapsed('libera/channels')).toEqual([])
    expect(sanitizeCollapsed(['a/b', 3, null, '', 'a/b', 'x'.repeat(301), 'c/d'])).toEqual([
      'a/b',
      'c/d',
    ])
    const many = Array.from({ length: maxCollapsedSections + 5 }, (_, i) => `n${i}/channels`)
    const kept = sanitizeCollapsed(many)
    expect(kept).toHaveLength(maxCollapsedSections)
    // The newest are kept, so collapsing one more section forgets the oldest.
    expect(kept.at(-1)).toBe(many.at(-1))
    expect(kept).not.toContain(many[0])
    expect(toggleCollapsed(kept, 'new/channels').at(-1)).toBe('new/channels')
  })

  test('a collapsed section keeps the open conversation and what wants attention', () => {
    const items = [
      { id: 'quiet', unreadCount: 0, mentionCount: 0 },
      { id: 'open', unreadCount: 0, mentionCount: 0 },
      { id: 'unread', unreadCount: 3, mentionCount: 0 },
      { id: 'muted', unreadCount: 3, mentionCount: 0 },
      { id: 'mentioned-muted', unreadCount: 1, mentionCount: 1 },
    ]
    const shown = collapsedItems(items, 'open', (c) => c.id.includes('muted'))
    expect(shown.map((c) => c.id)).toEqual(['open', 'unread', 'mentioned-muted'])
  })
})
