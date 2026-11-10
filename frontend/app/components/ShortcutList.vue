<script setup vapor lang="ts">
import {
  chordLabel,
  channelShortcuts,
  chordOf,
  chordUsers,
  reservedReason,
  sameChord,
  shortcutActionLabel,
  shortcutActions,
  shortcutGroupLabel,
  shortcutGroups,
  type Chord,
  type ShortcutGroup,
  type ShortcutName,
} from '../../shared/utils/shortcuts'
/**
 * Every keyboard shortcut, grouped. Settings shows it with `editable`, where you add, remove and
 * reset the keys of each action; the “?” overlay shows it read-only.
 *
 * The actions and their keys come from shared/utils/shortcuts.ts with your own bindings applied
 * (useShortcuts), the same table the handlers match against. Keys handled inside one widget
 * (Enter and Tab in the composer, Esc in pickers and menus, Shift F10 for context menus, held
 * Shift on the message toolbar) can't be changed and are listed here by hand.
 */
const { t } = useI18n()
const $t = t
const translate = (key: string) => t(key)
const props = withDefaults(defineProps<{ editable?: boolean }>(), { editable: false })
const emit = defineEmits<{ customize: [] }>()
const shortcutKeys = useShortcuts()
const mac = shortcutKeys.mac
const keys = (...chords: Chord[]) =>
  chords.map((chord) => chordLabel(chord, mac, translate)).join(' · ')
const availableChannelKeys = computed(() =>
  channelShortcuts.filter((chord) => !chordUsers(shortcutKeys.bindings.value, chord).length),
)
const fixed = computed<Record<ShortcutGroup, [string, string][]>>(() => ({
  'Getting around': [
    ...(availableChannelKeys.value.length
      ? [[t('shortcuts.items.channel'), keys(...availableChannelKeys.value)] as [string, string]]
      : []),
    [t('shortcuts.items.history'), t('shortcuts.values.mouseBackForward')],
  ],
  Reading: [
    [
      t('shortcuts.items.toolbarDelete'),
      t('shortcuts.values.hold', { key: chordLabel({ key: 'Shift' }, mac, translate) }),
    ],
    [
      t('shortcuts.items.contextMenu'),
      t('shortcuts.values.contextMenu', { keys: keys({ key: 'F10', shift: true }) }),
    ],
    [t('shortcuts.items.resize'), '← →'],
  ],
  Writing: [
    [t('shortcuts.items.send'), keys({ key: 'Enter' })],
    [t('shortcuts.items.newLine'), keys({ key: 'Enter', shift: true })],
    [t('mediaUi.shortcuts.fixedEditLast'), keys({ key: 'ArrowUp' })],
    [t('shortcuts.items.complete'), keys({ key: 'Tab' })],
    [t('shortcuts.items.triggers'), '/ · : · @ · #'],
    [t('shortcuts.items.cancel'), keys({ key: 'Escape' })],
  ],
  App: [],
}))
const labelOf = (name: ShortcutName) => shortcutActionLabel(name, translate)
const groups = computed(() =>
  shortcutGroups.map((id) => ({
    id,
    title: shortcutGroupLabel(id, translate),
    actions: shortcutActions.filter(
      (action) =>
        action.group === id &&
        (props.editable || shortcutKeys.bindings.value[action.name].length > 0),
    ),
    fixed: fixed.value[id],
  })),
)

// ---- Changing keys ----------------------------------------------------------------------------

/** The action getting new keys, and whether key presses are being recorded for it now. */
const recording = ref<ShortcutName | null>(null)
const capturing = ref(false)
/** A key already used by other actions, waiting for you to move it or try other keys. */
const pending = ref<{ chord: Chord; users: ShortcutName[] } | null>(null)
const reservedChord = ref<Chord | null>(null)
type Announcement =
  | { kind: 'pressKeysFor'; action: ShortcutName }
  | { kind: 'added' | 'removed' | 'nowRuns'; action: ShortcutName; chord: string }
  | { kind: 'defaultAgain'; action: ShortcutName }
  | { kind: 'allDefaultsAgain' | 'cancelled' }
  | { kind: 'reserved'; chord: Chord }
  | { kind: 'collision'; chord: Chord; users: ShortcutName[] }
const announcementState = ref<Announcement | null>(null)
const announcement = computed(() => {
  const state = announcementState.value
  if (!state) return ''
  if (state.kind === 'pressKeysFor')
    return t('mediaUi.shortcuts.pressKeysFor', { action: labelOf(state.action) })
  if (state.kind === 'added' || state.kind === 'removed' || state.kind === 'nowRuns')
    return t(`mediaUi.shortcuts.${state.kind}`, {
      action: labelOf(state.action),
      chord: state.chord,
    })
  if (state.kind === 'defaultAgain')
    return t('mediaUi.shortcuts.defaultAgain', { action: labelOf(state.action) })
  if (state.kind === 'allDefaultsAgain') return t('mediaUi.shortcuts.allDefaultsAgain')
  if (state.kind === 'cancelled') return t('mediaUi.shortcuts.cancelled')
  if (state.kind === 'reserved')
    return t('mediaUi.shortcuts.reserved', {
      chord: chordLabel(state.chord, mac, translate),
      reason: reservedReason(state.chord, translate) ?? '',
    })
  if (state.kind === 'collision')
    return t('mediaUi.shortcuts.collision', {
      chord: chordLabel(state.chord, mac, translate),
      actions: state.users.map(labelOf).join(', '),
    })
  return ''
})
const problem = computed(() => {
  if (reservedChord.value)
    return t('mediaUi.shortcuts.reserved', {
      chord: chordLabel(reservedChord.value, mac, translate),
      reason: reservedReason(reservedChord.value, translate) ?? '',
    })
  if (pending.value)
    return t('mediaUi.shortcuts.collision', {
      chord: chordLabel(pending.value.chord, mac, translate),
      actions: pending.value.users.map(labelOf).join(', '),
    })
  return ''
})
const root = ref<HTMLElement | null>(null)
/** Focuses the first of `selectors` in the action's row that exists, else the row itself. */
function focusIn(name: ShortcutName, ...selectors: string[]) {
  nextTick(() => {
    const row = root.value?.querySelector<HTMLElement>(`[data-action="${name}"]`)
    const target = selectors
      .map((selector) => row?.querySelector<HTMLElement>(selector))
      .find(Boolean)
    ;(target ?? row)?.focus()
  })
}
/** After a change: Add when the action can take more keys, else Reset, else the row. */
const focusAfterChange = (name: ShortcutName) => focusIn(name, '.shortcut-add', '.shortcut-reset')

function record(name: ShortcutName) {
  recording.value = name
  capturing.value = true
  pending.value = null
  reservedChord.value = null
  announcementState.value = { kind: 'pressKeysFor', action: name }
  focusIn(name, '.shortcut-recording')
}
function stop(focusAdd = false) {
  const name = recording.value
  recording.value = null
  capturing.value = false
  pending.value = null
  reservedChord.value = null
  if (focusAdd && name) focusAfterChange(name)
}
function add(name: ShortcutName, chord: Chord) {
  const own = shortcutKeys.bindings.value[name]
  if (!own.some((c) => sameChord(c, chord))) shortcutKeys.set(name, [...own, chord])
  announcementState.value = {
    kind: 'added',
    action: name,
    chord: chordLabel(chord, mac, translate),
  }
  stop(true)
}
/** Moves the pending key from the actions that had it to the one getting new keys. */
function move() {
  const name = recording.value
  const waiting = pending.value
  if (!name || !waiting) return
  const changes: Partial<Record<ShortcutName, Chord[]>> = {}
  for (const user of waiting.users)
    changes[user] = shortcutKeys.bindings.value[user].filter((c) => !sameChord(c, waiting.chord))
  changes[name] = [...shortcutKeys.bindings.value[name], waiting.chord]
  shortcutKeys.setMany(changes)
  announcementState.value = {
    kind: 'nowRuns',
    action: name,
    chord: chordLabel(waiting.chord, mac, translate),
  }
  stop(true)
}
function remove(name: ShortcutName, chord: Chord) {
  shortcutKeys.set(
    name,
    shortcutKeys.bindings.value[name].filter((c) => !sameChord(c, chord)),
  )
  announcementState.value = {
    kind: 'removed',
    action: name,
    chord: chordLabel(chord, mac, translate),
  }
  focusAfterChange(name)
}
function reset(name: ShortcutName) {
  shortcutKeys.reset(name)
  announcementState.value = { kind: 'defaultAgain', action: name }
  focusAfterChange(name)
}
const { confirm } = usePrompt()
async function resetAll() {
  stop()
  const ok = await confirm({
    title: t('mediaUi.shortcuts.resetAll.title'),
    body: t('mediaUi.shortcuts.resetAll.body'),
    confirm: t('mediaUi.shortcuts.resetAll.confirm'),
    danger: true,
  })
  if (!ok) return
  shortcutKeys.resetAll()
  announcementState.value = { kind: 'allDefaultsAgain' }
}

/** While recording, every key press is the new shortcut; nothing else acts on it. */
function capture(event: KeyboardEvent) {
  const name = recording.value
  if (!name || !capturing.value || event.isComposing || event.keyCode === 229) return
  // Esc cancels, and Tab leaves the recorder as it leaves any other control.
  const bare = !event.ctrlKey && !event.metaKey && !event.altKey
  if (bare && ((event.key === 'Escape' && !event.shiftKey) || event.key === 'Tab')) {
    // Esc would close Settings too.
    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopImmediatePropagation()
    }
    stop()
    announcementState.value = { kind: 'cancelled' }
    return
  }
  event.preventDefault()
  event.stopImmediatePropagation()
  if (event.repeat) return
  const chord = chordOf(event)
  if (!chord) return
  pending.value = null
  const reason = reservedReason(chord, translate)
  if (reason) {
    reservedChord.value = chord
    announcementState.value = { kind: 'reserved', chord }
    return
  }
  const users = chordUsers(shortcutKeys.bindings.value, chord, name)
  if (users.length) {
    // Stop recording so the choice can be made with the keyboard too.
    capturing.value = false
    pending.value = { chord, users }
    announcementState.value = { kind: 'collision', chord, users }
    focusIn(name, '.shortcut-move')
    return
  }
  add(name, chord)
}
watch(
  capturing,
  (on) => {
    if (on) window.addEventListener('keydown', capture, true)
    else window.removeEventListener('keydown', capture, true)
  },
  { flush: 'sync' },
)
onBeforeUnmount(() => window.removeEventListener('keydown', capture, true))
</script>
<template>
  <div
    ref="root"
    class="shortcut-groups grid gap-5"
    data-setting="keyboard-shortcuts"
  >
    <SettingNote v-if="editable">
      <span class="flex-1">{{ $t('mediaUi.shortcuts.instruction') }}</span>
      <UiButton
        v-if="shortcutKeys.anyChanged.value"
        class="shortcut-reset-all shrink-0"
        variant="secondary"
        size="xs"
        @click="resetAll"
      >
        {{ $t('mediaUi.shortcuts.resetAll.confirm') }}
      </UiButton>
    </SettingNote>
    <i18n-t
      v-else
      keypath="mediaUi.shortcuts.inSettings"
      tag="p"
      class="m-0 text-[13px] text-muted-foreground"
    >
      <template #action>
        <UiButton
          class="shortcut-customize"
          variant="inline"
          size="inline"
          @click="emit('customize')"
          >{{ $t('mediaUi.shortcuts.change') }}</UiButton
        >
      </template>
    </i18n-t>
    <section
      v-for="group in groups"
      :key="group.id"
    >
      <SettingsSubtitle>
        {{ group.title }}
      </SettingsSubtitle>
      <dl
        class="shortcut-list grid gap-0.5 mt-2 mr-0 mb-0 ml-0 [&>div]:flex [&>div]:flex-wrap [&>div]:items-center [&>div]:justify-between [&>div]:gap-x-3 [&>div]:gap-y-1.5 [&>div]:rounded-sm [&>div]:px-3 [&>div]:py-2 [&>div]:text-[14px] [&>div]:text-body [&>div:nth-child(odd)]:bg-card [&_dd]:m-0"
      >
        <div
          v-for="action in group.actions"
          :key="action.name"
          class="shortcut-action"
          tabindex="-1"
          :data-action="action.name"
        >
          <dt>{{ labelOf(action.name) }}</dt>
          <dd
            v-if="!editable"
            class="flex flex-wrap justify-end gap-1"
          >
            <kbd
              v-for="chord in shortcutKeys.bindings.value[action.name]"
              :key="chordLabel(chord, mac, translate)"
              >{{ chordLabel(chord, mac, translate) }}</kbd
            >
          </dd>
          <dd
            v-else
            class="flex flex-wrap items-center justify-end gap-1"
          >
            <span
              v-if="!shortcutKeys.bindings.value[action.name].length && recording !== action.name"
              class="shortcut-none text-[13px] text-muted-foreground"
              >{{ $t('mediaUi.shortcuts.notSet') }}</span
            >
            <span
              v-for="chord in shortcutKeys.bindings.value[action.name]"
              :key="chordLabel(chord, mac, translate)"
              class="shortcut-chord inline-flex items-center gap-0.5"
              ><kbd>{{ chordLabel(chord, mac, translate) }}</kbd
              ><UiButton
                variant="ghost"
                size="icon-xs"
                :aria-label="
                  $t('mediaUi.shortcuts.removeChord', {
                    chord: chordLabel(chord, mac, translate),
                    action: labelOf(action.name),
                  })
                "
                @click="remove(action.name, chord)"
                ><AppIcon
                  name="close"
                  :size="12" /></UiButton
            ></span>
            <UiButton
              v-if="recording === action.name"
              class="shortcut-recording"
              variant="default"
              size="xs"
              :aria-label="$t('mediaUi.shortcuts.recordingAria', { action: labelOf(action.name) })"
              @click="stop(true)"
              @blur="capturing && stop()"
            >
              {{ capturing ? $t('mediaUi.shortcuts.pressKeys') : $t('mediaUi.shortcuts.cancel') }}
            </UiButton>
            <UiButton
              v-else-if="shortcutKeys.bindings.value[action.name].length < 3"
              class="shortcut-add"
              variant="secondary"
              size="xs"
              :aria-label="$t('mediaUi.shortcuts.addAria', { action: labelOf(action.name) })"
              @click="record(action.name)"
            >
              {{ $t('mediaUi.shortcuts.add') }}
            </UiButton>
            <UiButton
              v-if="shortcutKeys.changed(action.name)"
              class="shortcut-reset"
              variant="ghost"
              size="xs"
              :aria-label="$t('mediaUi.shortcuts.resetAria', { action: labelOf(action.name) })"
              @click="reset(action.name)"
            >
              {{ $t('mediaUi.shortcuts.reset') }}
            </UiButton>
          </dd>
          <p
            v-if="editable && recording === action.name && problem"
            class="shortcut-problem basis-full m-0 flex flex-wrap items-center justify-end gap-2 text-[12px] text-muted-foreground"
            role="alert"
          >
            {{ problem }}
            <template v-if="pending">
              <UiButton
                class="shortcut-move"
                variant="secondary"
                size="xs"
                @mousedown.prevent
                @click="move"
              >
                {{ $t('mediaUi.shortcuts.useHere') }}
              </UiButton>
              <UiButton
                class="shortcut-retry"
                variant="ghost"
                size="xs"
                @mousedown.prevent
                @click="record(action.name)"
              >
                {{ $t('mediaUi.shortcuts.tryOtherKeys') }}
              </UiButton>
            </template>
          </p>
        </div>
        <div
          v-for="[label, combo] in group.fixed"
          :key="label"
        >
          <dt>{{ label }}</dt>
          <dd>
            <kbd>{{ combo }}</kbd>
          </dd>
        </div>
      </dl>
    </section>
    <span
      class="sr-only"
      role="status"
      >{{ announcement }}</span
    >
  </div>
</template>
