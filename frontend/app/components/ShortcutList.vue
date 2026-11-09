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
const { t } = useI18n()
const mac = isMac(navigator.platform)
const key = (name: keyof typeof shortcuts) => shortcutLabel(name, mac)
const keys = (...chords: Chord[]) => chords.map((chord) => chordLabel(chord, mac)).join(' · ')
/** “Alt ↑ / ↓”: two chords that differ only in their arrow. */
const pair = (back: keyof typeof shortcuts, forward: keyof typeof shortcuts) =>
  `${chordLabel(shortcuts[back][0]!, mac)} / ${chordLabel({ key: shortcuts[forward][0]!.key }, mac)}`
// Chord labels are platform symbols, so only the words around them are catalog text.
const groups = computed(() => [
  {
    title: t('shortcuts.groups.gettingAround'),
    items: [
      [t('shortcuts.items.search'), key('search')],
      [t('shortcuts.items.conversation'), pair('previous', 'next')],
      [t('shortcuts.items.channel'), `${chordLabel(shortcuts.channel[0]!, mac)}–9`],
      [t('shortcuts.items.unread'), pair('previousUnread', 'nextUnread')],
      [t('shortcuts.items.history'), t('shortcuts.values.mouseBackForward')],
    ],
  },
  {
    title: t('shortcuts.groups.reading'),
    items: [
      [t('shortcuts.items.markRead'), key('markRead')],
      [t('shortcuts.items.markNetworkRead'), key('markNetworkRead')],
      [
        t('shortcuts.items.toolbarDelete'),
        t('shortcuts.values.hold', { key: chordLabel({ key: 'Shift' }, mac) }),
      ],
      [
        t('shortcuts.items.contextMenu'),
        t('shortcuts.values.contextMenu', { keys: keys({ key: 'F10', shift: true }) }),
      ],
      [t('shortcuts.items.resize'), '← →'],
    ],
  },
  {
    title: t('shortcuts.groups.writing'),
    items: [
      [t('shortcuts.items.send'), keys({ key: 'Enter' })],
      [t('shortcuts.items.newLine'), keys({ key: 'Enter', shift: true })],
      [t('shortcuts.items.gifPicker'), key('gifs')],
      [t('shortcuts.items.complete'), keys({ key: 'Tab' })],
      [t('shortcuts.items.triggers'), '/ · : · @ · #'],
      [t('shortcuts.items.cancel'), keys({ key: 'Escape' })],
    ],
  },
  {
    title: t('shortcuts.groups.formatting'),
    items: [
      [t('shortcuts.items.bold'), key('bold')],
      [t('shortcuts.items.italic'), key('italic')],
      [t('shortcuts.items.underline'), key('underline')],
    ],
  },
  {
    title: t('shortcuts.groups.app'),
    items: [
      [t('shortcuts.items.settings'), key('settings')],
      [t('shortcuts.items.keyboardList'), key('help')],
    ],
  },
])
</script>
<template>
  <div
    class="shortcut-groups grid gap-5"
    data-setting="keyboard-shortcuts"
  >
    <section
      v-for="group in groups"
      :key="group.title"
    >
      <SettingsSubtitle>
        {{ group.title }}
      </SettingsSubtitle>
      <dl
        class="shortcut-list grid gap-0.5 mt-2 mr-0 mb-0 ml-0 [&>div]:flex [&>div]:items-center [&>div]:justify-between [&>div]:gap-3 [&>div]:rounded-sm [&>div]:px-3 [&>div]:py-2 [&>div]:text-[14px] [&>div]:text-body [&>div:nth-child(odd)]:bg-card [&_dd]:m-0"
      >
        <div
          v-for="[label, combo] in group.items"
          :key="label"
        >
          <dt>{{ label }}</dt>
          <dd>
            <kbd>{{ combo }}</kbd>
          </dd>
        </div>
      </dl>
    </section>
  </div>
</template>
