<script setup lang="ts">
/** Settings → Networks: every network with its connection, plus irc:// link handling. */
const emit = defineEmits<{ addNetwork: []; editNetwork: [id: string] }>()
const chat = useChatStore()
const preferences = usePreferencesStore()
const handlerRegistered = ref(false)
function registerHandler() {
  handlerRegistered.value = registerIrcHandler()
  if (!handlerRegistered.value) chat.notify('This browser doesn’t let sites open irc:// links.')
}
</script>
<template>
  <div>
    <div
      v-for="network in chat.data?.networks"
      :key="network.id"
      class="network-setting"
    >
      <img
        v-if="preferences.preferences.networkIcons[network.id]"
        class="network-mini"
        :src="preferences.preferences.networkIcons[network.id]"
        alt=""
      /><span
        v-else
        class="network-mini"
        :class="`tone-${network.color}`"
        >{{ network.initials }}</span
      ><span class="network-setting-name"
        ><strong>{{ network.name }}</strong
        ><small
          >{{ network.nick }} · {{ network.host }}:{{ network.port
          }}{{ network.tls ? ' · TLS' : '' }}</small
        ><small
          ><i
            class="status-dot"
            :class="`is-${network.status}`"
          />{{ network.status
          }}{{ network.auth.method !== 'none' ? ` · ${network.auth.method}` : '' }}</small
        ></span
      ><button
        class="button secondary small"
        @click="emit('editNetwork', network.id)"
      >
        Edit</button
      ><button
        class="button secondary small"
        :disabled="chat.connection.isLoading"
        @click="chat.connection.mutate(network)"
      >
        {{
          network.status === 'connected' || network.status === 'connecting'
            ? 'Disconnect'
            : 'Connect'
        }}
      </button>
    </div>
    <button
      class="button secondary add-network-setting"
      @click="emit('addNetwork')"
    >
      <AppIcon
        name="plus"
        :size="17"
      />Add a network
    </button>
    <section class="setting-group irc-links">
      <h3>irc:// links</h3>
      <div class="setting-row">
        <span class="setting-copy"
          ><strong>Open IRC links with IRCord</strong
          ><small>{{
            handlerRegistered
              ? 'Your browser will ask you to confirm. Links like irc://irc.libera.chat/#rust then open here.'
              : 'Links like irc://irc.libera.chat/#rust open here, and offer to add networks you don’t have yet.'
          }}</small></span
        >
        <button
          class="button secondary small"
          @click="registerHandler"
        >
          Use IRCord
        </button>
      </div>
    </section>
  </div>
</template>
