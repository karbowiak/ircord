<script setup lang="ts">
const props = defineProps<{ memberId: string }>()
const emit = defineEmits<{ close: []; edit: [] }>()
const chat = useChatStore()
const member = computed(() => chat.member(props.memberId))
const nameStyle = useNameStyle()
const error = ref('')
async function message() {
  try {
    await chat.direct.mutateAsync(props.memberId)
    emit('close')
  } catch (e) {
    error.value = errorText(e)
  }
}
/** Channel buttons stop their click, so the capture phase sees it first. */
function closeOnChannel(event: MouseEvent) {
  if ((event.target as HTMLElement | null)?.closest('.inline-channel')) emit('close')
}
</script>
<template>
  <AppDialog
    :title="member?.displayName || 'Profile'"
    @close="emit('close')"
    ><div
      v-if="member"
      class="profile-card"
    >
      <div
        class="profile-banner"
        :class="`tone-${member.color}`"
      />
      <UserAvatar
        :member="member"
        size="large"
        presence
      />
      <div class="profile-card-body">
        <div class="profile-title">
          <h3 :style="nameStyle(member)">{{ member.displayName }}</h3>
          <span
            v-if="member.role === 'bot'"
            class="bot-badge"
            >BOT</span
          ><AppIcon
            v-if="member.role === 'operator'"
            name="shield"
            :size="18"
          />
        </div>
        <p class="profile-nick">
          @{{ member.nick
          }}<span
            ><i
              class="status-dot"
              :class="{
                'is-offline': member.status === 'offline',
                'is-away': member.status === 'away',
              }"
            />{{ member.status }}</span
          >
        </p>
        <!-- A #channel button opens the channel behind the dialog, so the dialog steps aside. -->
        <p
          class="profile-bio"
          @click.capture="closeOnChannel"
        >
          <MessageRuns :text="member.bio ?? ''" />
        </p>
        <dl class="profile-details">
          <div>
            <dt>Network</dt>
            <dd>{{ chat.network?.name }}</dd>
          </div>
          <div>
            <dt>Account</dt>
            <dd>{{ member.account }}</dd>
          </div>
          <div v-if="member.location">
            <dt>Location</dt>
            <dd>{{ member.location }}</dd>
          </div>
        </dl>
        <p
          v-if="error"
          class="form-error"
          role="alert"
        >
          {{ error }}
        </p>
        <button
          v-if="member.id === 'you'"
          class="button primary full-width"
          @click="emit('edit')"
        >
          <AppIcon
            name="settings"
            :size="17"
          />Edit profile</button
        ><button
          v-else
          class="button primary full-width"
          :disabled="chat.direct.isLoading"
          @click="message"
        >
          <AppIcon
            name="message"
            :size="17"
          />{{ chat.direct.isLoading ? 'Opening…' : 'Message' }}
        </button>
      </div>
    </div></AppDialog
  >
</template>
