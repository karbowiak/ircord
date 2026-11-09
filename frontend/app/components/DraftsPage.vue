<script setup lang="ts">
import type { Conversation } from '../../shared/types/chat'
import { draftCount, draftRows, type DraftRow } from '../../shared/utils/drafts'
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
const total = computed(() => draftCount(chat.drafts))
const icon = (row: DraftRow) =>
  row.kind === 'server' ? 'terminal' : row.kind === 'direct' ? 'message' : 'hash'
const title = (row: DraftRow) =>
  row.kind === 'channel' && row.state === 'available' ? `#${row.title}` : row.title
/** “#linux on OFTC”: same-named channels on different networks stay apart for screen readers. */
const place = (row: DraftRow) =>
  row.networkName ? `${title(row)} on ${row.networkName}` : title(row)
const edited = (row: DraftRow) => {
  if (!row.editedAt) return 'Edited earlier'
  const iso = new Date(row.editedAt).toISOString()
  return `${day(iso)} ${time(iso)}`
}

/** The last discard, kept so it can be undone (or copied) while this list is open. */
const discarded = ref<{ row: DraftRow; refused?: 'conflict' | 'full' } | null>(null)
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
  discarded.value = { row }
}
function undo() {
  const last = discarded.value
  if (!last) return
  const result = chat.restoreDraft(last.row.id, last.row.text, last.row.editedAt)
  // Never overwrite newer text typed there meanwhile; offer the discarded text to copy instead.
  discarded.value = result === 'restored' ? null : { ...last, refused: result }
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
    ...(event.currentTarget as HTMLElement).querySelectorAll<HTMLElement>(
      '.draft-open:not(:disabled)',
    ),
  ]
  const i = list.indexOf(document.activeElement as HTMLElement)
  if (i < 0) return
  event.preventDefault()
  list[(i + (event.key === 'ArrowDown' ? 1 : -1) + list.length) % list.length]?.focus()
}
</script>
<template>
  <WorkspacePage
    title="Drafts"
    icon="pencil"
    description="Pick up where you left off. Drafts stay in this browser, on this device."
    :meta="total ? `${total} unsent` : undefined"
    @close="emit('close')"
  >
    <p
      class="drafts-scope m-0 flex items-start gap-2 px-4 pt-1 pb-3 text-[length:var(--ui-small)] text-muted-foreground max-[700px]:px-3"
    >
      <AppIcon
        name="info"
        :size="14"
        class="mt-0.5 flex-none"
      />Messages waiting to send and attachments are not listed here.
    </p>
    <UiAlert
      v-if="!chat.draftsSaved"
      as="p"
      variant="destructive"
      presentation="inline"
      class="form-error drafts-warning mx-4 mt-0 mb-2"
    >
      This browser isn’t saving drafts right now (storage is full or blocked). They stay here until
      this tab closes.
    </UiAlert>
    <div
      class="drafts-list min-h-0 flex-1 overflow-y-auto px-4 pb-4 max-[700px]:px-1.5"
      @keydown="rowKeys"
    >
      <div class="w-full space-y-0.5">
        <EmptyState v-if="!rows.length">
          <AppIcon
            name="pencil"
            :size="27"
          />
          <p>No drafts. Text you type and don’t send is kept here, per conversation.</p>
        </EmptyState>
        <p
          v-if="total > rows.length"
          class="list-label m-0 flex h-7 items-center px-3 text-[length:var(--ui-caption)] font-semibold uppercase tracking-[0.02em] text-muted-foreground"
        >
          Showing the newest {{ rows.length }} of {{ total }} drafts
        </p>
        <div
          v-for="row in rows"
          :key="row.id"
          class="draft-row group flex flex-wrap items-start gap-1 rounded-md transition-colors hover:bg-accent/60 focus-within:bg-accent/60"
          :class="{
            'is-active': row.id === chat.activeId,
            'is-unavailable': row.state !== 'available',
          }"
        >
          <button
            class="draft-open flex flex-1 min-w-0 items-start gap-3.5 px-3 py-2.5 rounded-md text-left disabled:cursor-default disabled:opacity-100"
            :disabled="row.state !== 'available'"
            @click="open(row)"
          >
            <span
              class="search-result-icon mt-0.5 flex size-10 shrink-0 items-center justify-center rounded-md bg-secondary text-muted-foreground"
              ><AppIcon
                :name="icon(row)"
                :size="18" /></span
            ><span class="draft-text flex-1 min-w-0 flex flex-col gap-0.5"
              ><span
                class="draft-location flex flex-wrap items-baseline gap-x-2 gap-y-0.5 text-[12px] text-muted-foreground"
                ><strong
                  class="text-[15px] leading-[1.375] font-semibold"
                  :class="row.state === 'available' ? 'text-foreground' : 'text-muted-foreground'"
                  >{{ title(row) }}</strong
                ><span v-if="row.networkName">{{ row.networkName }}</span
                ><time
                  v-if="row.editedAt"
                  class="ml-auto pl-2 text-faint tabular-nums whitespace-nowrap"
                  :datetime="new Date(row.editedAt).toISOString()"
                  >{{ edited(row) }}</time
                ><small
                  v-else
                  class="draft-time ml-auto pl-2 text-faint"
                  >{{ edited(row) }}</small
                ></span
              ><span
                class="draft-preview text-[length:var(--ui-text)] leading-normal text-body overflow-hidden whitespace-pre-line wrap-anywhere [display:-webkit-box] [-webkit-line-clamp:3] [-webkit-box-orient:vertical]"
                >{{ row.text.slice(0, 360) }}</span
              ><small
                v-if="row.state === 'unavailable'"
                class="draft-stranded text-[12px] text-muted-foreground"
                >This conversation is no longer in your workspace. The text is kept until you remove
                it.</small
              ><small
                v-else-if="row.state === 'unknown'"
                class="draft-stranded text-[12px] text-muted-foreground"
                >Conversations are still loading.</small
              ></span
            >
          </button>
          <span
            class="draft-actions flex gap-0.5 pr-2 pt-2 max-[700px]:[&_[data-slot=button]]:min-h-10 max-[700px]:[&_[data-slot=button]]:min-w-10"
          >
            <UiButton
              v-if="row.state === 'unavailable'"
              variant="ghost"
              size="icon-sm"
              :aria-label="`${inspecting === row.id ? 'Hide' : 'Show'} full draft in ${place(row)}`"
              :aria-expanded="inspecting === row.id"
              :title="inspecting === row.id ? 'Hide full draft' : 'Show full draft'"
              @click="inspecting = inspecting === row.id ? '' : row.id"
            >
              <AppIcon
                name="search"
                :size="16" /></UiButton
            ><UiButton
              variant="ghost"
              size="icon-sm"
              :aria-label="`Copy draft in ${place(row)}`"
              title="Copy draft"
              @click="copy(row.text)"
            >
              <AppIcon
                name="copy"
                :size="16" /></UiButton
            ><UiButton
              variant="ghost"
              size="icon-sm"
              class="draft-discard hover:not-disabled:text-destructive focus-visible:text-destructive"
              :aria-label="`Discard draft in ${place(row)}`"
              title="Discard draft"
              @click="discard(row)"
            >
              <AppIcon
                name="trash"
                :size="16"
              />
            </UiButton>
          </span>
          <UiTextarea
            v-if="inspecting === row.id"
            class="draft-full mt-0 mr-2 mb-2 ml-[66px] min-h-24 basis-full p-2 text-body max-[700px]:ml-2"
            readonly
            :model-value="row.text"
            :aria-label="`Full draft in ${place(row)}`"
          />
        </div>
      </div>
    </div>
    <div
      class="drafts-status flex flex-wrap items-center gap-[4px_12px] min-h-[41px] py-2.5 px-4 border-t [border-top-style:solid] border-t-border/40 text-[13px] text-muted-foreground max-[700px]:px-3 empty:min-h-0 empty:border-t-0 empty:p-0"
      role="status"
    >
      <template v-if="discarded"
        ><span v-if="discarded.refused === 'conflict'"
          >{{ title(discarded.row) }} has a newer draft, so the discarded text wasn’t put
          back.</span
        ><span v-else-if="discarded.refused === 'full'"
          >This device already keeps as many drafts as it can, so the discarded text wasn’t put
          back.</span
        ><span v-else>Draft in {{ title(discarded.row) }} discarded.</span
        ><UiButton
          v-if="discarded.refused"
          variant="link"
          size="text"
          @click="copy(discarded.row.text)"
        >
          Copy discarded text</UiButton
        ><UiButton
          v-else
          variant="link"
          size="text"
          @click="undo"
        >
          Undo
        </UiButton></template
      ><span v-if="notice">{{ notice }}</span>
    </div>
  </WorkspacePage>
</template>
