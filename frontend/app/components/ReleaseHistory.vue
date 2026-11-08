<script setup lang="ts">
import { releaseLabel } from '../../shared/utils/changelog'

/**
 * The persistent entry point: the release this page runs and every earlier release the deployed
 * build knows. When the server already runs a newer release than this page, that one and anything
 * in between are listed too and labelled, so the running and available versions never blur.
 */
const changelog = useChangelog()
const update = useAppUpdate()
onMounted(changelog.load)
const running = changelog.running.version
const deployed = computed(() => changelog.data.value?.version ?? '')
/** The server runs a different build than this page (a reload is waiting). */
const behind = computed(
  () => !!changelog.data.value && changelog.data.value.buildId !== changelog.running.buildId,
)
const badges = { running: 'Running', available: 'Available', new: 'Not installed yet', '': '' }
</script>
<template>
  <section
    class="release-history"
    aria-labelledby="release-history-title"
  >
    <h4 id="release-history-title">Release notes</h4>
    <p
      class="settings-description"
      role="status"
    >
      <template v-if="running"
        >You’re running IRCord <strong>{{ running }}</strong
        >.</template
      >
      <template v-else>You’re running a development build of IRCord.</template>
      <template v-if="behind && deployed && deployed !== running"
        >{{ ' ' }}<strong>{{ deployed }}</strong> is available; reload to update.</template
      >
      <template v-else-if="behind">{{ ' ' }}A newer build is available; reload to update.</template>
    </p>
    <button
      v-if="behind"
      class="button primary small"
      @click="update.reload()"
    >
      Reload now
    </button>
    <p
      v-if="changelog.status.value === 'loading' && !changelog.data.value"
      class="settings-description"
    >
      Loading release notes…
    </p>
    <template v-else-if="changelog.status.value === 'failed'">
      <p class="settings-description">Couldn’t load the release notes.</p>
      <button
        class="button secondary small"
        @click="changelog.load()"
      >
        Try again
      </button>
    </template>
    <p
      v-else-if="!changelog.data.value?.releases.length"
      class="settings-description"
    >
      No release notes are available for this build.
    </p>
    <template v-else>
      <ReleaseEntry
        v-for="release in changelog.data.value.releases"
        :key="release.version"
        :release="release"
        :badge="badges[releaseLabel(release.version, running, deployed)]"
      />
    </template>
  </section>
</template>
