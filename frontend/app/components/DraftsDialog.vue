<script setup lang="ts">
import type { Conversation } from '../../shared/types/chat'
import { draftRows, type DraftRow } from '../../shared/utils/drafts'
const emit = defineEmits<{ close: []; open: [conversationId: string] }>()
const chat = useChatStore()
const { day, time } = useTimeFormat()

const label = (c: Conversation) =>
  c.kind === 'direct' ? (chat.directPartner(c)?.displayName ?? c.name) : c.name
const rows = computed(() =>
  draftRows(
    chat.drafts,
    chat.draftTimes,
    chat.data && { conversations: chat.data.conversations, networks: chat.data.networks },
    label,
  ),
)
const icon = (row: DraftRow) =>
  row.kind === 'server' ? 'terminal' : row.kind === 'direct' ? 'message' : 'hash'
const title = (row: DraftRow) =>
  row.kind === 'channel' && row.state !== 'unavailable' ? `#${row.title}` : row.title
/** “#linux on OFTC”: same-named channels on different networks stay apart for screen readers. */
const place = (row: DraftRow) =>
  row.networkName ? `${title(row)} on ${row.networkName}` : title(row)
const edited = (row: DraftRow) => {
  if (!row.editedAt) return 'Edited earlier'
  const iso = new Date(row.editedAt).toISOString()
  return `${day(iso)} ${time(iso)}`
}

/** The last discard, kept so it can be undone (or copied) while this list is open. */
const discarded = ref<{ row: DraftRow; conflict: boolean } | null>(null)
const notice = ref('')
/** A stranded draft opened for reading and copying. */
const inspecting = ref('')

function open(row: DraftRow) {
  if (row.state !== 'available') return
  emit('open', row.id)
}
function discard(row: DraftRow) {
  notice.value = ''
  if (!chat.discardDraft(row.id, row.text)) {
    notice.value = 'That draft changed since it was listed, so it was kept. Check it and try again.'
    return
  }
  if (inspecting.value === row.id) inspecting.value = ''
  discarded.value = { row, conflict: false }
}
function undo() {
  const last = discarded.value
  if (!last) return
  if (chat.restoreDraft(last.row.id, last.row.text, last.row.editedAt)) {
    discarded.value = null
    return
  }
  // Something new was typed there meanwhile: never overwrite it.
  discarded.value = { ...last, conflict: true }
}
async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    notice.value = 'Copied the draft.'
  } catch {
    notice.value = 'Couldn’t copy to the clipboard. Select the text and copy it instead.'
  }
}
function rowKeys(event: KeyboardEvent) {
  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
  const list = [
    ...(event.currentTarget as HTMLElement).querySelectorAll<HTMLElement>('.draft-open:not(:disabled)'),
  ]
  const i = list.indexOf(document.activeElement as HTMLElement)
  if (i < 0) return
  event.preventDefault()
  list[(i + (event.key === 'ArrowDown' ? 1 : -1) + list.length) % list.length]?.focus()
}
</script>
<template>
  <AppDialog
    title="Drafts"
    wide
    @close="emit('close')"
  >
    <p class="drafts-scope">
      <AppIcon
        name="pencil"
        :size="14"
      />Unsent text on this device only. Other devices and browsers keep their own drafts;
      unsent messages and attachments are not listed here.
    </p>
    <p
      v-if="!chat.draftsSaved"
      class="form-error drafts-warning"
      role="alert"
    >
      This browser isn’t saving drafts right now (storage is full or blocked). They stay here until
      this tab closes.
    </p>
    <div
      class="drafts-list"
      @keydown="rowKeys"
    >
      <div
        v-if="!rows.length"
        class="search-empty"
      >
        <AppIcon
          name="pencil"
          :size="27"
        />
        <p>No drafts. Text you type and don’t send is kept here, per conversation.</p>
      </div>
      <div
        v-for="row in rows"
        :key="row.id"
        class="draft-row"
        :class="{ 'is-active': row.id === chat.activeId, 'is-unavailable': row.state !== 'available' }"
      >
        <button
          class="draft-open"
          :disabled="row.state !== 'available'"
          @click="open(row)"
        >
          <span class="search-result-icon"><AppIcon :name="icon(row)" /></span
          ><span class="draft-text"
            ><span class="draft-location"
              ><strong>{{ title(row) }}</strong
              ><small v-if="row.networkName">{{ row.networkName }}</small
              ><small v-if="row.id === chat.activeId">· Open now</small
              ><time
                v-if="row.editedAt"
                :datetime="new Date(row.editedAt).toISOString()"
                >{{ edited(row) }}</time
              ><small
                v-else
                class="draft-time"
                >{{ edited(row) }}</small
              ></span
            ><span class="draft-preview">{{ row.preview }}</span
            ><small
              v-if="row.state === 'unavailable'"
              class="draft-stranded"
              >This conversation is no longer in your workspace. The text is kept until you remove
              it.</small
            ><small
              v-else-if="row.state === 'unknown'"
              class="draft-stranded"
              >Conversations are still loading.</small
            ></span
          >
        </button>
        <span class="draft-actions">
          <button
            v-if="row.state === 'unavailable'"
            class="icon-button"
            :aria-label="`${inspecting === row.id ? 'Hide' : 'Show'} full draft in ${place(row)}`"
            :aria-expanded="inspecting === row.id"
            :title="inspecting === row.id ? 'Hide full draft' : 'Show full draft'"
            @click="inspecting = inspecting === row.id ? '' : row.id"
          >
            <AppIcon
              name="search"
              :size="16"
            /></button
          ><button
            class="icon-button"
            :aria-label="`Copy draft in ${place(row)}`"
            title="Copy draft"
            @click="copy(row.text)"
          >
            <AppIcon
              name="copy"
              :size="16"
            /></button
          ><button
            class="icon-button draft-discard"
            :aria-label="`Discard draft in ${place(row)}`"
            title="Discard draft"
            @click="discard(row)"
          >
            <AppIcon
              name="trash"
              :size="16"
            />
          </button>
        </span>
        <textarea
          v-if="inspecting === row.id"
          class="draft-full"
          readonly
          :value="row.text"
          :aria-label="`Full draft in ${place(row)}`"
        />
      </div>
    </div>
    <div
      class="drafts-status"
      role="status"
    >
      <template v-if="discarded"
        ><span v-if="discarded.conflict"
          >{{ title(discarded.row) }} has a newer draft, so the discarded text wasn’t put back.</span
        ><span v-else>Draft in {{ title(discarded.row) }} discarded.</span
        ><button
          v-if="discarded.conflict"
          class="text-button"
          @click="copy(discarded.row.text)"
        >
          Copy discarded text</button
        ><button
          v-else
          class="text-button"
          @click="undo"
        >
          Undo
        </button></template
      ><span v-if="notice">{{ notice }}</span>
    </div>
  </AppDialog>
</template>
