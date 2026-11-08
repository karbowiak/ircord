<script setup lang="ts">
import { compareVersions, parseVersion, releaseLabel } from '../../shared/utils/changelog'

/**
 * The persistent entry point: the release this page runs and every earlier release the deployed
 * build knows. When the server offers another build (the one the update banner announced), that
 * release and anything in between are listed too and labelled, so the running and available
 * versions never blur. Only an announced build counts as an update, as in the banner; an older
 * replica answering mid-rollout is not one.
 */
const changelog = useChangelog()
const update = useAppUpdate()
onMounted(() => {
  void changelog.load()
  void update.check()
})
const running = changelog.running.version
const data = computed(() => changelog.data.value)
/** The server runs the build the update banner announced, not this page's build. */
const offered = computed(
  () =>
    !!data.value &&
    data.value.buildId !== changelog.running.buildId &&
    data.value.buildId === update.latestId,
)
const deployed = computed(() => (offered.value ? data.value!.version : ''))
const rollback = computed(
  () =>
    offered.value &&
    !!parseVersion(running) &&
    !!parseVersion(deployed.value) &&
    compareVersions(deployed.value, running) < 0,
)
/** The running release is a version the server's data does not describe. */
const runningMissing = computed(
  () =>
    !!data.value?.releases.length &&
    !!parseVersion(running) &&
    !data.value.releases.some((r) => r.version === running),
)
const badges = { running: 'Running', available: 'Available', new: 'Not installed yet', '': '' }
</script>
<template>
  <section
    class="release-history grid gap-3 [justify-items:start] w-full mt-3 pt-4 border-t [border-top-style:solid] border-t-border"
    aria-labelledby="release-history-title"
  >
    <h4
      id="release-history-title"
      tabindex="-1"
    >
      Release notes
    </h4>
    <SettingsDescription role="status">
      <template v-if="running"
        >You’re running IRCord <strong>{{ running }}</strong
        >.</template
      >
      <template v-else>You’re running a development build of IRCord.</template>
      <template v-if="rollback"
        >{{ ' ' }}IRCord was rolled back to <strong>{{ deployed }}</strong
        >; reload to switch to it.</template
      >
      <template v-else-if="offered && deployed && deployed !== running"
        >{{ ' ' }}<strong>{{ deployed }}</strong> is available; reload to update.</template
      >
      <template v-else-if="offered"
        >{{ ' ' }}A newer build is available; reload to update.</template
      >
    </SettingsDescription>
    <UiButton
      size="sm"
      v-if="offered"
      :variant="rollback ? 'secondary' : 'default'"
      @click="update.reload()"
    >
      {{ rollback ? `Switch to ${deployed}` : 'Reload now' }}
    </UiButton>
    <SettingsDescription v-if="changelog.status.value === 'loading' && !data">
      Loading release notes…
    </SettingsDescription>
    <template v-else-if="changelog.status.value === 'failed'">
      <SettingsDescription> Couldn’t load the release notes. </SettingsDescription>
      <UiButton
        variant="secondary"
        size="sm"
        @click="changelog.load()"
      >
        Try again
      </UiButton>
    </template>
    <SettingsDescription v-else-if="!data?.releases.length">
      No release notes are available for this build.
    </SettingsDescription>
    <template v-else>
      <SettingsDescription v-if="runningMissing">
        The server has no notes for {{ running }}, the release you’re running.
      </SettingsDescription>
      <ReleaseEntry
        v-for="release in data.releases"
        :key="release.version"
        :release="release"
        :level="5"
        :badge="badges[releaseLabel(release.version, running, deployed)]"
      />
    </template>
  </section>
</template>
