<script setup lang="ts">
import {
  chordLabel,
  chordOf,
  chordUsers,
  reservedReason,
  sameChord,
  shortcutActions,
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
const props = withDefaults(defineProps<{ editable?: boolean }>(), { editable: false })
const emit = defineEmits<{ customize: [] }>()
const shortcutKeys = useShortcuts()
const mac = shortcutKeys.mac
const keys = (...chords: Chord[]) => chords.map((chord) => chordLabel(chord, mac)).join(' · ')
const fixed: Record<ShortcutGroup, [string, string][]> = {
  'Getting around': [['Back / forward through conversations', 'Mouse back / forward']],
  Reading: [
    ['Mark as unread · delete (message toolbar)', `Hold ${chordLabel({ key: 'Shift' }, mac)}`],
    ['Context menu', `Right-click · ${keys({ key: 'F10', shift: true })} · long-press`],
    ['Resize a focused panel edge', '← →'],
  ],
  Writing: [
    ['Send', keys({ key: 'Enter' })],
    ['New line', keys({ key: 'Enter', shift: true })],
    ['Edit your last message (empty message box)', keys({ key: 'ArrowUp' })],
    ['Complete a nickname', keys({ key: 'Tab' })],
    ['Commands · emoji · people · channels', '/ · : · @ · #'],
    ['Cancel reply or close a picker or menu', keys({ key: 'Escape' })],
  ],
  App: [],
}
const groups = computed(() =>
  shortcutGroups.map((title) => ({
    title,
    actions: shortcutActions.filter(
      (action) =>
        action.group === title &&
        (props.editable || shortcutKeys.bindings.value[action.name].length > 0),
    ),
    fixed: fixed[title],
  })),
)
const labelOf = (name: ShortcutName) => shortcutActions.find((a) => a.name === name)?.label ?? name

// ---- Changing keys ----------------------------------------------------------------------------

/** The action getting new keys, and whether key presses are being recorded for it now. */
const recording = ref<ShortcutName | null>(null)
const capturing = ref(false)
const problem = ref('')
/** A key already used by other actions, waiting for you to move it or try other keys. */
const pending = ref<{ chord: Chord; users: ShortcutName[] } | null>(null)
const announcement = ref('')
const root = ref<HTMLElement | null>(null)
function focusIn(name: ShortcutName, selector: string) {
  nextTick(() =>
    root.value?.querySelector<HTMLElement>(`[data-action="${name}"] ${selector}`)?.focus(),
  )
}

function record(name: ShortcutName) {
  recording.value = name
  capturing.value = true
  problem.value = ''
  pending.value = null
  announcement.value = `Press the keys for ${labelOf(name)}. Press Tab to cancel.`
  focusIn(name, '.shortcut-recording')
}
function stop(focusAdd = false) {
  const name = recording.value
  recording.value = null
  capturing.value = false
  problem.value = ''
  pending.value = null
  if (focusAdd && name) focusIn(name, '.shortcut-add')
}
function add(name: ShortcutName, chord: Chord) {
  const own = shortcutKeys.bindings.value[name]
  if (!own.some((c) => sameChord(c, chord))) shortcutKeys.set(name, [...own, chord])
  announcement.value = `${labelOf(name)}: ${chordLabel(chord, mac)} added.`
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
  announcement.value = `${chordLabel(waiting.chord, mac)} now runs ${labelOf(name)}.`
  stop(true)
}
function remove(name: ShortcutName, chord: Chord) {
  shortcutKeys.set(
    name,
    shortcutKeys.bindings.value[name].filter((c) => !sameChord(c, chord)),
  )
  announcement.value = `${labelOf(name)}: ${chordLabel(chord, mac)} removed.`
  focusIn(name, '.shortcut-add')
}
function reset(name: ShortcutName) {
  shortcutKeys.reset(name)
  announcement.value = `${labelOf(name)} uses its default keys again.`
  focusIn(name, '.shortcut-add')
}
function resetAll() {
  stop()
  shortcutKeys.resetAll()
  announcement.value = 'All shortcuts use their default keys again.'
}

/** While recording, every key press is the new shortcut; nothing else acts on it. */
function capture(event: KeyboardEvent) {
  const name = recording.value
  if (!name || !capturing.value || event.isComposing || event.keyCode === 229) return
  // Tab leaves the recorder, as it leaves any other control.
  if (event.key === 'Tab' && !event.ctrlKey && !event.metaKey && !event.altKey) {
    stop()
    announcement.value = 'Cancelled.'
    return
  }
  event.preventDefault()
  event.stopImmediatePropagation()
  if (event.repeat) return
  const chord = chordOf(event)
  if (!chord) return
  pending.value = null
  const reason = reservedReason(chord)
  if (reason) {
    problem.value = `${chordLabel(chord, mac)} can’t be a shortcut. ${reason}`
    announcement.value = problem.value
    return
  }
  const users = chordUsers(shortcutKeys.bindings.value, chord, name)
  if (users.length) {
    // Stop recording so the choice can be made with the keyboard too.
    capturing.value = false
    pending.value = { chord, users }
    problem.value = `${chordLabel(chord, mac)} is used by ${users.map(labelOf).join(', ')}.`
    announcement.value = problem.value
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
      <span class="flex-1"
        >Choose Add to give an action more keys. Press the new keys, or Tab to cancel. Your
        shortcuts follow your account.</span
      >
      <UiButton
        v-if="shortcutKeys.anyChanged.value"
        class="shortcut-reset-all shrink-0"
        variant="secondary"
        size="xs"
        @click="resetAll"
      >
        Reset all
      </UiButton>
    </SettingNote>
    <p
      v-else
      class="m-0 text-[13px] text-muted-foreground"
    >
      <UiButton
        class="shortcut-customize"
        variant="inline"
        size="inline"
        @click="emit('customize')"
        >Change shortcuts</UiButton
      >
      in Settings.
    </p>
    <section
      v-for="group in groups"
      :key="group.title"
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
          :data-action="action.name"
        >
          <dt>{{ action.label }}</dt>
          <dd
            v-if="!editable"
            class="flex flex-wrap justify-end gap-1"
          >
            <kbd
              v-for="chord in shortcutKeys.bindings.value[action.name]"
              :key="chordLabel(chord, mac)"
              >{{ chordLabel(chord, mac) }}</kbd
            >
          </dd>
          <dd
            v-else
            class="flex flex-wrap items-center justify-end gap-1"
          >
            <span
              v-if="!shortcutKeys.bindings.value[action.name].length && recording !== action.name"
              class="shortcut-none text-[13px] text-muted-foreground"
              >Not set</span
            >
            <span
              v-for="chord in shortcutKeys.bindings.value[action.name]"
              :key="chordLabel(chord, mac)"
              class="shortcut-chord inline-flex items-center gap-0.5"
              ><kbd>{{ chordLabel(chord, mac) }}</kbd
              ><UiButton
                variant="ghost"
                size="icon-xs"
                :aria-label="`Remove ${chordLabel(chord, mac)} from ${action.label}`"
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
              :aria-label="`Recording keys for ${action.label}. Choose to cancel.`"
              @click="stop(true)"
              @blur="capturing && stop()"
            >
              {{ capturing ? 'Press keys…' : 'Cancel' }}
            </UiButton>
            <UiButton
              v-else-if="shortcutKeys.bindings.value[action.name].length < 3"
              class="shortcut-add"
              variant="secondary"
              size="xs"
              :aria-label="`Add keys for ${action.label}`"
              @click="record(action.name)"
            >
              Add
            </UiButton>
            <UiButton
              v-if="shortcutKeys.changed(action.name)"
              class="shortcut-reset"
              variant="ghost"
              size="xs"
              :aria-label="`Reset ${action.label} to its default keys`"
              @click="reset(action.name)"
            >
              Reset
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
                Use here
              </UiButton>
              <UiButton
                class="shortcut-retry"
                variant="ghost"
                size="xs"
                @mousedown.prevent
                @click="record(action.name)"
              >
                Try other keys
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
