<script setup lang="ts">
/** Every keyboard shortcut, grouped. Shown in Settings and in the “?” overlay. */
const mac = /Mac|iPhone|iPad/.test(navigator.platform)
const mod = mac ? '⌘' : 'Ctrl'
const groups = [
  {
    title: 'Getting around',
    items: [
      ['Search and quick switcher', `${mod} K`],
      ['Previous / next conversation', 'Alt ↑ / ↓'],
      ['Next unread or mention', 'Alt Shift ↑ / ↓'],
      ['Back / forward through conversations', 'Mouse back / forward'],
      ['Settings', `${mod} ,`],
      ['This list', `? · ${mod} /`],
    ],
  },
  {
    title: 'Reading',
    items: [
      ['Mark conversation as read', 'Esc'],
      ['Mark network as read', 'Shift Esc'],
      ['Context menu', 'Right-click · Shift F10 · long-press'],
      ['Resize a focused panel edge', '← →'],
    ],
  },
  {
    title: 'Writing',
    items: [
      ['Send', 'Enter'],
      ['New line', 'Shift Enter'],
      ['Bold · italic · underline', `${mod} B · ${mod} I · ${mod} U`],
      ['Complete a nickname', 'Tab'],
      ['Commands · emoji · people · channels', '/ · : · @ · #'],
      ['Cancel reply or close a menu', 'Esc'],
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
