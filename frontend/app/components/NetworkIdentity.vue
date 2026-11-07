<script setup lang="ts">
/**
 * Live identity on the current network: nickname, away state and (where the server supports
 * IRCv3 setname) real name. Changes are sent as IRC commands; the backend applies them.
 */
const chat = useChatStore()
const network = computed(() => chat.network)
const nick = ref(network.value?.nick ?? '')
const realname = ref(network.value?.realname ?? '')
const awayMessage = ref('')
const away = computed(() => chat.data?.user.status === 'away')
watch(network, (n) => {
  nick.value = n?.nick ?? ''
  realname.value = n?.realname ?? ''
})
function changeNick() {
  if (nick.value.trim() && nick.value !== network.value?.nick) chat.raw(`NICK ${nick.value.trim()}`)
}
function changeRealname() {
  if (realname.value.trim()) chat.raw(`SETNAME :${realname.value.trim()}`)
}
function toggleAway() {
  chat.raw(away.value ? 'AWAY' : `AWAY :${awayMessage.value.trim() || 'Away'}`)
}
</script>
<template>
  <section
    v-if="network"
    class="network-identity-settings"
  >
    <h4 class="settings-subtitle">On {{ network.name }}</h4>
    <form
      class="inline-field"
      @submit.prevent="changeNick"
    >
      <label
        class="field-label"
        for="identity-nick"
        >Nickname</label
      >
      <div>
        <input
          id="identity-nick"
          v-model="nick"
          class="text-input"
          maxlength="31"
          autocomplete="off"
        /><button
          class="button secondary small"
          :disabled="!nick.trim() || nick === network.nick"
        >
          Change
        </button>
      </div>
    </form>
    <form
      v-if="network.features.setname"
      class="inline-field"
      @submit.prevent="changeRealname"
    >
      <label
        class="field-label"
        for="identity-realname"
        >Real name</label
      >
      <div>
        <input
          id="identity-realname"
          v-model="realname"
          class="text-input"
          maxlength="100"
          autocomplete="off"
        /><button
          class="button secondary small"
          :disabled="!realname.trim() || realname === network.realname"
        >
          Change
        </button>
      </div>
    </form>
    <p
      v-else
      class="field-hint"
    >
      This network can’t change your real name while connected; edit it in the network settings and
      reconnect.
    </p>
    <form
      class="inline-field"
      @submit.prevent="toggleAway"
    >
      <label
        class="field-label"
        for="identity-away"
        >Away</label
      >
      <div>
        <input
          id="identity-away"
          v-model="awayMessage"
          class="text-input"
          :placeholder="away ? 'You are away' : 'Away message (optional)'"
          :disabled="away"
          maxlength="200"
        /><button class="button secondary small">
          {{ away ? 'I’m back' : 'Set away' }}
        </button>
      </div>
    </form>
  </section>
</template>
