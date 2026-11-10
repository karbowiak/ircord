<script setup vapor lang="ts">
const chat = useChatStore()
const { t: $t } = useI18n()
const selectedNetworkId = ref(chat.networkId || chat.data?.networks[0]?.id || '')
watch(
  () => chat.data?.networks,
  (networks) => {
    if (!networks?.some((network) => network.id === selectedNetworkId.value))
      selectedNetworkId.value = networks?.[0]?.id || ''
  },
)
</script>

<template>
  <div class="profile-settings">
    <div
      v-if="chat.data?.networks.length"
      class="grid gap-2 mb-5"
    >
      <label for="profile-server">{{ $t('profile.server') }}</label>
      <select
        id="profile-server"
        v-model="selectedNetworkId"
        class="w-full rounded-md border border-border bg-background px-3 py-2 text-foreground"
      >
        <option
          v-for="network in chat.data.networks"
          :key="network.id"
          :value="network.id"
        >
          {{ network.name }}
        </option>
      </select>
    </div>
    <ProfileEditor
      v-if="selectedNetworkId"
      :key="selectedNetworkId"
      :network-id="selectedNetworkId"
    />
    <FieldHint v-else>{{ $t('profile.addServer') }}</FieldHint>
    <NetworkIdentity
      v-if="selectedNetworkId"
      :key="selectedNetworkId"
      :network-id="selectedNetworkId"
    />
  </div>
</template>
