<script setup vapor lang="ts">
/**
 * Settings → Networks: every network with its connection, registering channels you create, and
 * irc:// link handling.
 */
import { scopeLabels } from '../../shared/settings-index'

const emit = defineEmits<{ addNetwork: []; editNetwork: [id: string] }>()
const chat = useChatStore()
const preferences = usePreferencesStore()
const { t } = useI18n()
const $t = t
const handlerRegistered = ref(false)
function registerHandler() {
  handlerRegistered.value = registerIrcHandler()
  if (!handlerRegistered.value) chat.notify(t('networks.handlerUnsupported'))
}
const statusLabels = computed<Record<string, string>>(() => ({
  connected: t('networks.status.connected'),
  disconnected: t('networks.status.disconnected'),
  connecting: t('networks.status.connecting'),
  error: t('networks.status.error'),
}))
</script>
<template>
  <div data-setting="network-list">
    <div
      v-for="network in chat.data?.networks"
      :key="network.id"
      class="network-setting flex items-center gap-3 p-3 mb-2 rounded-md bg-card"
    >
      <NuxtImg
        v-if="preferences.preferences.networkIcons[network.id]"
        class="network-mini w-9 h-9 flex items-center justify-center rounded-md font-[650] object-cover"
        :src="preferences.preferences.networkIcons[network.id]"
        alt=""
      /><span
        v-else
        class="network-mini w-9 h-9 flex items-center justify-center rounded-md font-[650]"
        :class="`tone-${network.color}`"
        >{{ network.initials }}</span
      ><span
        class="network-setting-name flex-1 min-w-0 flex flex-col [&_small]:flex [&_small]:items-center [&_small]:gap-1.5 [&_small]:text-[12px] [&_small]:text-muted-foreground [&_small]:capitalize"
        ><strong class="text-[14px]">{{ network.name }}</strong
        ><small class="normal-case! text-faint!"
          >{{ network.nick }} · {{ network.host }}:{{ network.port
          }}{{ network.tls ? ' · TLS' : '' }}</small
        ><small
          ><i
            class="status-dot inline-block w-[7px] h-[7px] bg-success rounded-full shrink-0"
            :class="`is-${network.status}`"
          />{{ statusLabels[network.status] ?? network.status
          }}{{ network.auth.method !== 'none' ? ` · ${network.auth.method}` : '' }}</small
        ><small v-if="network.statusDetail && network.status === 'error'">{{
          network.statusDetail
        }}</small></span
      ><UiButton
        variant="secondary"
        size="sm"
        @click="emit('editNetwork', network.id)"
      >
        {{ $t('common.edit') }}</UiButton
      ><UiButton
        variant="secondary"
        size="sm"
        :disabled="chat.connection.isLoading"
        @click="chat.connection.mutate(network)"
      >
        {{
          network.status === 'connected'
            ? $t('networks.disconnect')
            : network.status === 'connecting'
              ? $t('networks.cancelConnection')
              : $t('networks.connect')
        }}
      </UiButton>
    </div>
    <UiButton
      variant="secondary"
      data-setting="add-network"
      class="add-network-setting mt-1"
      @click="emit('addNetwork')"
    >
      <AppIcon
        name="plus"
        :size="17"
      />{{ $t('networks.add') }}
    </UiButton>
    <SettingGroup
      :title="$t('networks.groups.channels')"
      :scope="$t(scopeLabels.account)"
    >
      <SettingRow
        data-setting="register-channels"
        :label="$t('networks.registerChannels')"
        :description="$t('networks.registerChannelsHelp')"
      >
        <AppSwitch
          :model-value="preferences.preferences.registerChannels"
          :aria-label="$t('networks.registerChannels')"
          @change="preferences.update({ registerChannels: $event })"
        />
      </SettingRow>
    </SettingGroup>
    <section class="setting-group grid gap-3.5 irc-links">
      <h3>
        <i18n-t keypath="mediaUi.networkSettings.ircLinksHeading">
          <template #scheme><code>irc://</code></template>
        </i18n-t>
      </h3>
      <div
        data-setting="irc-links"
        class="setting-row flex items-center justify-between gap-4"
      >
        <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
          ><strong>{{ $t('networks.openIrcLinks') }}</strong
          ><small>{{
            handlerRegistered
              ? $t('networks.ircLinksRegistered', { example: 'irc://irc.libera.chat/#rust' })
              : $t('networks.ircLinksDefault', { example: 'irc://irc.libera.chat/#rust' })
          }}</small></span
        >
        <UiButton
          variant="secondary"
          size="sm"
          @click="registerHandler"
        >
          {{ $t('networks.useIrcord') }}
        </UiButton>
      </div>
    </section>
  </div>
</template>
