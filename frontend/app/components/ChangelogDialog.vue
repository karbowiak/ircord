<script setup vapor lang="ts">
import { updateNotes } from '../../shared/utils/changelog'

/**
 * What a reload to the announced build brings: every release after the one this page runs, up to
 * the deployed one. Opening and closing it changes nothing else; while it is open the update's
 * overlay guard keeps a hidden tab from reloading underneath it.
 */
const props = defineProps<{ announced: string }>()
const emit = defineEmits<{ close: []; reload: [] }>()
const { t } = useI18n()
const $t = t
const changelog = useChangelog()
const notes = computed(() => updateNotes(changelog.running, changelog.data.value, props.announced))
const runningLabel = computed(() => changelog.running.version || t('release.developmentBuild'))
onMounted(changelog.load)
watch(() => props.announced, changelog.load)
</script>
<template>
  <AppDialog
    v-slot="{ close }"
    :title="$t('release.title')"
    wide
    @close="emit('close')"
  >
    <div
      class="dialog-form changelog pt-0 pr-5 pb-5 pl-5 grid gap-4 [&>.settings-description]:mb-3.5"
    >
      <SettingsDescription
        v-if="changelog.status.value === 'loading' && !changelog.data.value"
        role="status"
      >
        {{ $t('release.loading') }}
      </SettingsDescription>
      <template v-else-if="changelog.status.value === 'failed' || notes.kind === 'stale'">
        <SettingsDescription role="status">
          {{
            notes.kind === 'stale' && changelog.status.value !== 'failed'
              ? $t('release.rollingOut')
              : $t('release.unavailable')
          }}
        </SettingsDescription>
        <UiButton
          variant="secondary"
          class="justify-self-start"
          @click="changelog.load()"
        >
          {{ $t('common.tryAgain') }}
        </UiButton>
      </template>
      <template v-else>
        <SettingsDescription
          class="changelog-summary"
          role="status"
        >
          <template v-if="notes.kind === 'upgrade'">
            <i18n-t keypath="release.upgrade">
              <template #running
                ><strong>{{ runningLabel }}</strong></template
              >
              <template #target
                ><strong>{{ notes.target }}</strong></template
              >
            </i18n-t>
            <template v-if="notes.releases.length > 1">
              {{ $t('release.includesReleases', notes.releases.length) }}
            </template>
          </template>
          <i18n-t
            v-else-if="notes.kind === 'same'"
            keypath="release.same"
          >
            <template #target
              ><strong>{{ notes.target }}</strong></template
            >
          </i18n-t>
          <i18n-t
            v-else-if="notes.kind === 'rollback'"
            keypath="release.rolledBack"
          >
            <template #target
              ><strong>{{ notes.target }}</strong></template
            >
            <template #running
              ><strong>{{ runningLabel }}</strong></template
            >
          </i18n-t>
          <i18n-t
            v-else-if="notes.kind === 'unknown-running'"
            keypath="release.unknownRunning"
          >
            <template #target
              ><strong>{{ notes.target }}</strong></template
            >
          </i18n-t>
          <template v-else>{{ $t('release.noNotesForUpdate') }}</template>
        </SettingsDescription>
        <ReleaseEntry
          v-for="release in notes.releases"
          :key="release.version"
          :release="release"
        />
      </template>
      <!-- Closing goes through the dialog, which restores focus to the banner. -->
      <div
        class="form-actions changelog-actions flex justify-end items-center gap-2 sticky bottom-0 mt-0 mr-[-20px] mb-[-20px] ml-[-20px] pt-3 pr-5 pb-[calc(12px_+_env(safe-area-inset-bottom))] pl-5 bg-background border-t [border-top-style:solid] border-t-border"
      >
        <UiButton
          variant="secondary"
          type="button"
          @click="close"
        >
          {{ $t('common.close') }}
        </UiButton>
        <UiButton
          type="button"
          @click="emit('reload')"
        >
          {{ $t('common.reloadNow') }}
        </UiButton>
      </div>
    </div>
  </AppDialog>
</template>
