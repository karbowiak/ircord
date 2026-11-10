<script setup lang="ts" vapor>
import { draftCount, draftRows, type DraftRow } from '../../../shared/utils/drafts'
const emit = defineEmits<{ close: []; open: [conversationId: string] }>()
const chat = useChatStore()
const { day, time } = useTimeFormat()
const { t, n, locale } = useI18n()

const label = useSidebarOrder().name
const rows = computed(() =>
  draftRows(
    chat.drafts,
    chat.draftTimes,
    chat.data && { conversations: chat.data.conversations, networks: chat.data.networks },
    label,
    undefined,
    (key) => t(key),
    locale.value,
  ),
)
const total = computed(() => draftCount(chat.drafts))
const icon = (row: DraftRow) =>
  row.kind === 'server' ? 'terminal' : row.kind === 'direct' ? 'message' : 'hash'
const title = (row: DraftRow) =>
  row.kind === 'channel' && row.state === 'available' ? `#${row.title}` : row.title
/** “#linux on OFTC”: same-named channels on different networks stay apart for screen readers. */
const place = (row: DraftRow) =>
  row.networkName
    ? t('collectionsUi.drafts.onNetwork', { conversation: title(row), network: row.networkName })
    : title(row)
const edited = (row: DraftRow) => {
  if (!row.editedAt) return t('collectionsUi.drafts.editedEarlier')
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
    notice.value = 'changed'
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
    notice.value = 'copied'
  } catch {
    notice.value = 'copyFailed'
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
    :title="t('collectionsUi.drafts.title')"
    icon="pencil"
    :description="t('collectionsUi.drafts.description')"
    :meta="total ? t('collectionsUi.drafts.meta', { count: n(total) }, total) : undefined"
    @close="emit('close')"
  >
    <p
      class="drafts-scope m-0 flex items-start gap-2 px-4 pt-1 pb-3 text-[length:var(--ui-small)] text-muted-foreground max-[700px]:px-3"
    >
      <AppIcon
        name="info"
        :size="14"
        class="mt-0.5 flex-none"
      />{{ t('collectionsUi.drafts.scope') }}
    </p>
    <UiAlert
      v-if="!chat.draftsSaved"
      as="p"
      variant="destructive"
      presentation="inline"
      class="form-error drafts-warning mx-4 mt-0 mb-2"
    >
      {{ t('collectionsUi.drafts.storageWarning') }}
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
          <p>{{ t('collectionsUi.drafts.empty') }}</p>
        </EmptyState>
        <p
          v-if="total > rows.length"
          class="list-label m-0 flex h-7 items-center px-3 text-[length:var(--ui-caption)] font-semibold uppercase tracking-[0.02em] text-muted-foreground"
        >
          {{ t('collectionsUi.drafts.showing', { shown: n(rows.length), total: n(total) }) }}
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
                >{{ t('collectionsUi.drafts.unavailable') }}</small
              ><small
                v-else-if="row.state === 'unknown'"
                class="draft-stranded text-[12px] text-muted-foreground"
                >{{ t('collectionsUi.drafts.loadingConversations') }}</small
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
              :aria-label="
                t(
                  inspecting === row.id
                    ? 'collectionsUi.drafts.hideFull'
                    : 'collectionsUi.drafts.showFull',
                  { place: place(row) },
                )
              "
              :aria-expanded="inspecting === row.id"
              :title="
                t(
                  inspecting === row.id
                    ? 'collectionsUi.drafts.hideFullTitle'
                    : 'collectionsUi.drafts.showFullTitle',
                )
              "
              @click="inspecting = inspecting === row.id ? '' : row.id"
            >
              <AppIcon
                name="search"
                :size="16" /></UiButton
            ><UiButton
              variant="ghost"
              size="icon-sm"
              :aria-label="t('collectionsUi.drafts.copyIn', { place: place(row) })"
              :title="t('collectionsUi.drafts.copy')"
              @click="copy(row.text)"
            >
              <AppIcon
                name="copy"
                :size="16" /></UiButton
            ><UiButton
              variant="ghost"
              size="icon-sm"
              class="draft-discard hover:not-disabled:text-destructive focus-visible:text-destructive"
              :aria-label="t('collectionsUi.drafts.discardIn', { place: place(row) })"
              :title="t('collectionsUi.drafts.discard')"
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
            :aria-label="t('collectionsUi.drafts.fullIn', { place: place(row) })"
          />
        </div>
      </div>
    </div>
    <div
      class="drafts-status flex flex-wrap items-center gap-[4px_12px] min-h-[41px] py-2.5 px-4 border-t [border-top-style:solid] border-t-border/40 text-[13px] text-muted-foreground max-[700px]:px-3 empty:min-h-0 empty:border-t-0 empty:p-0"
      role="status"
    >
      <template v-if="discarded"
        ><span v-if="discarded.refused === 'conflict'">{{
          t('collectionsUi.drafts.discardConflict', { draft: title(discarded.row) })
        }}</span
        ><span v-else-if="discarded.refused === 'full'">{{
          t('collectionsUi.drafts.discardFull')
        }}</span
        ><span v-else>{{
          t('collectionsUi.drafts.discarded', { draft: title(discarded.row) })
        }}</span
        ><UiButton
          v-if="discarded.refused"
          variant="link"
          size="text"
          @click="copy(discarded.row.text)"
        >
          {{ t('collectionsUi.drafts.copyDiscarded') }}</UiButton
        ><UiButton
          v-else
          variant="link"
          size="text"
          @click="undo"
        >
          {{ t('collectionsUi.drafts.undo') }}
        </UiButton></template
      ><span v-if="notice">{{ t(`collectionsUi.drafts.${notice}`) }}</span>
    </div>
  </WorkspacePage>
</template>
