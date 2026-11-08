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
    <p
      class="settings-description text-[13px] text-muted-foreground leading-[1.55]"
      role="status"
    >
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
    </p>
    <button
      v-if="offered"
      class="button small"
      :class="rollback ? 'secondary' : 'primary'"
      @click="update.reload()"
    >
      {{ rollback ? `Switch to ${deployed}` : 'Reload now' }}
    </button>
    <p
      v-if="changelog.status.value === 'loading' && !data"
      class="settings-description text-[13px] text-muted-foreground leading-[1.55]"
    >
      Loading release notes…
    </p>
    <template v-else-if="changelog.status.value === 'failed'">
      <p class="settings-description text-[13px] text-muted-foreground leading-[1.55]">
        Couldn’t load the release notes.
      </p>
      <button
        class="button secondary small"
        @click="changelog.load()"
      >
        Try again
      </button>
    </template>
    <p
      v-else-if="!data?.releases.length"
      class="settings-description text-[13px] text-muted-foreground leading-[1.55]"
    >
      No release notes are available for this build.
    </p>
    <template v-else>
      <p
        v-if="runningMissing"
        class="settings-description text-[13px] text-muted-foreground leading-[1.55]"
      >
        The server has no notes for {{ running }}, the release you’re running.
      </p>
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
