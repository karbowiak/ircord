<script setup vapor lang="ts">
import { compareVersions, parseVersion, releaseLabel } from '../../shared/utils/changelog'

/**
 * The persistent entry point: the release this page runs and every earlier release the deployed
 * build knows. When the server offers another build (the one the update banner announced), that
 * release and anything in between are listed too and labelled, so the running and available
 * versions never blur. Only an announced build counts as an update, as in the banner; an older
 * replica answering mid-rollout is not one.
 */
const { t } = useI18n()
const $t = t
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
const badges = computed<Record<string, string>>(() => ({
  running: t('release.badge.running'),
  available: t('release.badge.available'),
  new: t('release.badge.new'),
  '': '',
}))
</script>
<template>
  <section
    class="release-history grid gap-3 [justify-items:start] w-full mt-3 pt-4 border-t [border-top-style:solid] border-t-border"
    aria-labelledby="release-history-title"
  >
    <h4
      id="release-history-title"
      class="scroll-mt-3 text-[14px] font-[650] focus:outline-none"
      tabindex="-1"
    >
      {{ $t('release.history') }}
    </h4>
    <SettingsDescription role="status">
      <i18n-t
        v-if="running"
        keypath="release.running"
      >
        <template #version
          ><strong>{{ running }}</strong></template
        >
      </i18n-t>
      <template v-else>{{ $t('release.runningDevelopment') }}</template>
      <template v-if="rollback">
        {{ ' ' }}
        <i18n-t keypath="release.rolledBackSwitch">
          <template #deployed
            ><strong>{{ deployed }}</strong></template
          >
        </i18n-t>
      </template>
      <template v-else-if="offered && deployed && deployed !== running">
        {{ ' ' }}
        <i18n-t keypath="release.available">
          <template #deployed
            ><strong>{{ deployed }}</strong></template
          >
        </i18n-t>
      </template>
      <template v-else-if="offered">{{ ' ' }}{{ $t('release.newerAvailable') }}</template>
    </SettingsDescription>
    <UiButton
      v-if="offered"
      size="sm"
      :variant="rollback ? 'secondary' : 'default'"
      @click="update.reload()"
    >
      {{ rollback ? $t('release.switchTo', { version: deployed }) : $t('common.reloadNow') }}
    </UiButton>
    <SettingsDescription v-if="changelog.status.value === 'loading' && !data">
      {{ $t('release.loading') }}
    </SettingsDescription>
    <template v-else-if="changelog.status.value === 'failed'">
      <SettingsDescription>{{ $t('release.failed') }}</SettingsDescription>
      <UiButton
        variant="secondary"
        size="sm"
        @click="changelog.load()"
      >
        {{ $t('common.tryAgain') }}
      </UiButton>
    </template>
    <SettingsDescription v-else-if="!data?.releases.length">
      {{ $t('release.noNotesForBuild') }}
    </SettingsDescription>
    <template v-else>
      <SettingsDescription v-if="runningMissing">
        {{ $t('release.runningMissing', { version: running }) }}
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
