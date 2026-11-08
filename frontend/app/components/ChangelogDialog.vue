<script setup lang="ts">
import { updateNotes } from '../../shared/utils/changelog'

/**
 * What a reload to the announced build brings: every release after the one this page runs, up to
 * the deployed one. Opening and closing it changes nothing else; while it is open the update's
 * overlay guard keeps a hidden tab from reloading underneath it.
 */
const props = defineProps<{ announced: string }>()
const emit = defineEmits<{ close: []; reload: [] }>()
const changelog = useChangelog()
const notes = computed(() => updateNotes(changelog.running, changelog.data.value, props.announced))
const runningLabel = computed(() => changelog.running.version || 'a development build')
onMounted(changelog.load)
watch(() => props.announced, changelog.load)
</script>
<template>
  <AppDialog
    title="What’s new in IRCord"
    wide
    @close="emit('close')"
  >
    <div class="dialog-form changelog pt-0 pr-5 pb-5 pl-5 grid gap-4">
      <p
        v-if="changelog.status.value === 'loading' && !changelog.data.value"
        class="settings-description text-[13px] text-muted-foreground leading-[1.55]"
        role="status"
      >
        Loading release notes…
      </p>
      <template v-else-if="changelog.status.value === 'failed' || notes.kind === 'stale'">
        <p
          class="settings-description text-[13px] text-muted-foreground leading-[1.55]"
          role="status"
        >
          {{
            notes.kind === 'stale' && changelog.status.value !== 'failed'
              ? 'The update is still rolling out, so its release notes aren’t available yet.'
              : 'Couldn’t load the release notes. You can still reload to update.'
          }}
        </p>
        <button
          class="button secondary"
          @click="changelog.load()"
        >
          Try again
        </button>
      </template>
      <template v-else>
        <p
          class="settings-description text-[13px] text-muted-foreground leading-[1.55] changelog-summary"
          role="status"
        >
          <template v-if="notes.kind === 'upgrade'"
            >Reload to update from <strong>{{ runningLabel }}</strong> to
            <strong>{{ notes.target }}</strong
            >.<template v-if="notes.releases.length > 1">
              That includes {{ notes.releases.length }} releases.</template
            ></template
          >
          <template v-else-if="notes.kind === 'same'"
            >This update rebuilds <strong>{{ notes.target }}</strong
            >, the release you’re running.</template
          >
          <template v-else-if="notes.kind === 'rollback'"
            >IRCord was rolled back to <strong>{{ notes.target }}</strong
            >. You’re running <strong>{{ runningLabel }}</strong
            >.</template
          >
          <template v-else-if="notes.kind === 'unknown-running'"
            >Reload to update to <strong>{{ notes.target }}</strong
            >. You’re running a development build, so only its own notes are shown.</template
          >
          <template v-else
            >Release notes aren’t available for this update. You can still reload to get
            it.</template
          >
        </p>
        <ReleaseEntry
          v-for="release in notes.releases"
          :key="release.version"
          :release="release"
        />
      </template>
      <!-- method="dialog" closes the native dialog, which restores focus to the banner. -->
      <form
        method="dialog"
        class="form-actions changelog-actions flex justify-end items-center gap-2 sticky bottom-0 mt-0 mr-[-20px] mb-[-20px] ml-[-20px] pt-3 pr-5 pb-[calc(12px_+_env(safe-area-inset-bottom))] pl-5 bg-background border-t [border-top-style:solid] border-t-border"
      >
        <button class="button secondary">Close</button>
        <button
          type="button"
          class="button primary"
          @click="emit('reload')"
        >
          Reload now
        </button>
      </form>
    </div>
  </AppDialog>
</template>
