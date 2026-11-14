<script setup vapor lang="ts">
import type { SegmentedOption } from '../shared/AppSegmented.vue'
import { channelTarget } from '../../../shared/utils/irc'

type Range = '1h' | '24h' | '7d'

const { t, n } = useI18n()
const chat = useChatStore()
const { current, close, settle } = useModeration()
const reason = ref('')
const deleteFiles = ref(false)
const range = ref<Range>('24h')
const preview = shallowRef<{ count: number; limit: number } | null>(null)
const previewError = ref('')
const starting = ref(false)
let previewRequest = 0

const ranges = computed<SegmentedOption<Range>[]>(() => [
  { value: '1h', label: t('workspaceUi.moderation.lastHour') },
  { value: '24h', label: t('workspaceUi.moderation.lastDay') },
  { value: '7d', label: t('workspaceUi.moderation.lastWeek') },
])

/** Counts the member's messages in the chosen range without deleting anything. */
async function loadPreview() {
  const request = current.value
  if (request?.kind !== 'recent') return
  const id = ++previewRequest
  preview.value = null
  previewError.value = ''
  try {
    const result = await apiClient().recentMessages(
      request.conversation.id,
      request.member.id,
      range.value,
    )
    if (id === previewRequest) preview.value = { count: result.count, limit: result.limit }
  } catch (error) {
    if (id === previewRequest)
      previewError.value =
        error instanceof Error ? error.message : t('workspaceUi.moderation.countFailed')
  }
}

watch(
  current,
  (request) => {
    reason.value = ''
    deleteFiles.value = false
    range.value = '24h'
    preview.value = null
    previewError.value = ''
    starting.value = false
    if (request?.kind === 'recent') void loadPreview()
  },
  { immediate: true },
)

const channel = computed(() =>
  current.value?.kind === 'recent' ? channelTarget(current.value.conversation) : '',
)

const messageBody = computed(() => {
  const request = current.value
  if (request?.kind !== 'message') return ''
  if (request.files && request.owner) return t('workspaceUi.moderation.deleteFilesOwnerBody')
  if (request.files) return t('workspaceUi.moderation.deleteFilesKeptBody')
  return t('workspaceUi.menus.deleteBody')
})

/** Polls the background job and reports the outcome as a notice. */
async function follow(jobId: string, nick: string) {
  for (;;) {
    await new Promise((resolve) => setTimeout(resolve, 1500))
    let job
    try {
      job = await apiClient().recentMessagesDeletion(jobId)
    } catch {
      return chat.notify(t('workspaceUi.moderation.progressLost', { nick }))
    }
    if (job.state !== 'done') continue
    const parts = [
      job.deleted === job.total
        ? t('workspaceUi.moderation.deletedAll', { count: n(job.deleted), nick }, job.deleted)
        : t('workspaceUi.moderation.deletedSome', {
            deleted: n(job.deleted),
            total: n(job.total),
            nick,
          }),
    ]
    if (job.tooOld)
      parts.push(t('workspaceUi.moderation.tooOld', { count: n(job.tooOld) }, job.tooOld))
    if (job.failed)
      parts.push(t('workspaceUi.moderation.notDeleted', { count: n(job.failed) }, job.failed))
    return chat.notify(parts.join(' '))
  }
}

async function submit() {
  const request = current.value
  if (!request) return
  if (request.kind === 'message') {
    settle({ reason: reason.value.trim(), deleteFiles: deleteFiles.value })
    return
  }
  if (!preview.value?.count || starting.value) return
  starting.value = true
  try {
    const job = await apiClient().deleteRecentMessages(request.conversation.id, request.member.id, {
      range: range.value,
      reason: request.owner ? reason.value.trim() || undefined : undefined,
    })
    close()
    if (!job.total)
      return chat.notify(t('workspaceUi.moderation.nothingToDelete', { nick: request.member.nick }))
    chat.notify(
      t(
        'workspaceUi.moderation.deleting',
        { count: n(job.total), nick: request.member.nick },
        job.total,
      ),
    )
    void follow(job.id, request.member.nick)
  } catch (error) {
    starting.value = false
    chat.notifyError(error)
  }
}
</script>
<template>
  <AppDialog
    v-if="current"
    :title="
      current.kind === 'message'
        ? t('workspaceUi.moderation.deleteTitle', { nick: current.nick })
        : t('workspaceUi.moderation.recentTitle', { nick: current.member.nick })
    "
    @close="close()"
  >
    <form
      class="dialog-form grid gap-4 pt-0 pr-5 pb-5 pl-5"
      @submit.prevent="submit"
    >
      <SettingsDescription v-if="current.kind === 'message'">
        {{ messageBody }}
      </SettingsDescription>
      <AppSegmented
        v-else
        v-model="range"
        :options="ranges"
        :label="t('workspaceUi.moderation.range')"
        @change="loadPreview()"
      />
      <AppField
        v-if="current.owner"
        id="moderation-reason"
        :label="t('workspaceUi.menus.reasonOptional')"
        :help="t('workspaceUi.moderation.reasonHelp')"
      >
        <template #default="control">
          <UiInput
            v-bind="control"
            v-model="reason"
            maxlength="300"
            autocomplete="off"
          />
        </template>
      </AppField>
      <label
        v-if="current.kind === 'message' && current.owner && current.files"
        class="flex items-center gap-2 text-[13px] text-body"
        ><UiCheckbox v-model="deleteFiles" />{{
          t('workspaceUi.moderation.alsoDeleteFiles')
        }}</label
      >
      <p
        v-if="current.kind === 'recent'"
        class="m-0 text-[13px] text-body"
        aria-live="polite"
      >
        <template v-if="previewError">{{ previewError }}</template>
        <template v-else-if="!preview">{{ t('workspaceUi.moderation.counting') }}</template>
        <template v-else-if="!preview.count">{{
          t('workspaceUi.moderation.noMessages', { nick: current.member.nick })
        }}</template>
        <template v-else>
          {{
            t(
              'workspaceUi.moderation.willDelete',
              { count: n(preview.count), channel },
              preview.count,
            )
          }}
          <template v-if="preview.count >= preview.limit">
            {{ t('workspaceUi.moderation.limit', { count: n(preview.limit) }) }}
          </template>
          {{ t('workspaceUi.moderation.cannotUndo') }}
        </template>
      </p>
      <p
        v-if="current.owner"
        class="m-0 text-[12px] text-muted-foreground"
      >
        {{ t('workspaceUi.moderation.recorded') }}
      </p>
      <div class="form-actions flex justify-end items-center gap-2 mt-1">
        <UiButton
          variant="secondary"
          type="button"
          @click="close()"
        >
          {{ t('common.cancel') }}
        </UiButton>
        <UiButton
          variant="destructive"
          :disabled="current.kind === 'recent' && (!preview?.count || starting)"
        >
          {{
            current.kind === 'message'
              ? t('workspaceUi.message.delete')
              : t(
                  'workspaceUi.moderation.deleteCount',
                  { count: n(preview?.count ?? 0) },
                  preview?.count ?? 0,
                )
          }}
        </UiButton>
      </div>
    </form>
  </AppDialog>
</template>
