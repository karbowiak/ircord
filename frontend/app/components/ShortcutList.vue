<script setup lang="ts">
import {
  chordLabel,
  isMac,
  shortcutLabel,
  shortcuts,
  type Chord,
} from '../../shared/utils/shortcuts'
/**
 * Every keyboard shortcut, grouped. Shown in Settings and in the “?” overlay.
 *
 * The global and formatting shortcuts come from shared/utils/shortcuts.ts, the definitions their
 * handlers match against. Keys handled inside one widget (Enter and Tab in the composer, Esc in
 * pickers and menus, Shift F10 for context menus, held Shift on the message toolbar) are listed
 * here by hand; all of them are rendered through the same platform-aware labels.
 */
const mac = isMac(navigator.platform)
const key = (name: keyof typeof shortcuts) => shortcutLabel(name, mac)
const keys = (...chords: Chord[]) => chords.map((chord) => chordLabel(chord, mac)).join(' · ')
/** “Alt ↑ / ↓”: two chords that differ only in their arrow. */
const pair = (back: keyof typeof shortcuts, forward: keyof typeof shortcuts) =>
  `${chordLabel(shortcuts[back][0]!, mac)} / ${chordLabel({ key: shortcuts[forward][0]!.key }, mac)}`
const groups = [
  {
    title: 'Getting around',
    items: [
      ['Search and quick switcher', key('search')],
      ['Previous / next conversation', pair('previous', 'next')],
      ['Next unread or mention', pair('previousUnread', 'nextUnread')],
      ['Back / forward through conversations', 'Mouse back / forward'],
    ],
  },
  {
    title: 'Reading',
    items: [
      ['Mark conversation as read', key('markRead')],
      ['Mark network as read', key('markNetworkRead')],
      ['Mark as unread · delete (message toolbar)', `Hold ${chordLabel({ key: 'Shift' }, mac)}`],
      ['Context menu', `Right-click · ${keys({ key: 'F10', shift: true })} · long-press`],
      ['Resize a focused panel edge', '← →'],
    ],
  },
  {
    title: 'Writing',
    items: [
      ['Send', keys({ key: 'Enter' })],
      ['New line', keys({ key: 'Enter', shift: true })],
      ['GIF picker', key('gifs')],
      ['Complete a nickname', keys({ key: 'Tab' })],
      ['Commands · emoji · people · channels', '/ · : · @ · #'],
      ['Cancel reply or close a picker or menu', keys({ key: 'Escape' })],
    ],
  },
  {
    title: 'Formatting',
    items: [
      ['Bold', key('bold')],
      ['Italic', key('italic')],
      ['Underline', key('underline')],
    ],
  },
  {
    title: 'App',
    items: [
      ['Settings', key('settings')],
      ['Keyboard shortcuts (this list)', key('help')],
    ],
  },
]
</script>
<template>
  <div
    class="shortcut-groups"
    data-setting="keyboard-shortcuts"
  >
    <section
      v-for="group in groups"
      :key="group.title"
    >
      <h4 class="settings-subtitle">{{ group.title }}</h4>
      <dl class="shortcut-list">
        <div
          v-for="[label, keys] in group.items"
          :key="label"
        >
          <dt>{{ label }}</dt>
          <dd>
            <kbd>{{ keys }}</kbd>
          </dd>
        </div>
      </dl>
    </section>
  </div>
</template>
