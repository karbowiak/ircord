<script setup lang="ts">
import { chordLabel, isMac, shortcutLabel, shortcuts } from '../../shared/utils/shortcuts'
/**
 * Every keyboard shortcut, grouped. Shown in Settings and in the “?” overlay. Keys that IRCord
 * handles itself come from shared/utils/shortcuts.ts, the same definitions the handlers use.
 */
const mac = isMac(navigator.platform)
const key = (name: keyof typeof shortcuts) => shortcutLabel(name, mac)
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
      ['Context menu', 'Right-click · Shift F10 · long-press'],
      ['Resize a focused panel edge', '← →'],
    ],
  },
  {
    title: 'Writing',
    items: [
      ['Send', 'Enter'],
      ['New line', 'Shift Enter'],
      ['GIF picker', key('gifs')],
      ['Complete a nickname', 'Tab'],
      ['Commands · emoji · people · channels', '/ · : · @ · #'],
      ['Cancel reply or close a picker or menu', 'Esc'],
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
  <div class="shortcut-groups">
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
