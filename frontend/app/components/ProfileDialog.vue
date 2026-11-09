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
      class="profile-card relative"
    >
      <div
        class="profile-banner h-24 my-0 mx-3 rounded-md"
        :class="`tone-${member.color}`"
      />
      <div
        class="absolute top-13 left-7 flex rounded-full shadow-[0_0_0_6px_var(--main)] [&_.presence]:border-[var(--main)]"
      >
        <UserAvatar
          :member="member"
          size="large"
          presence
        />
      </div>
      <div class="profile-card-body pt-13 pr-5 pb-5 pl-5">
        <div class="profile-title flex items-center gap-2">
          <h3
            class="text-[20px] font-[700]"
            :style="nameStyle(member)"
          >
            {{ member.displayName }}
          </h3>
          <span
            v-if="member.role === 'bot'"
            class="bot-badge inline-flex items-center h-4 py-0 px-[5px] rounded-[4px] bg-selection text-primary text-[10px] font-[650] tracking-[0.04em] shrink-0"
            >BOT</span
          ><AppIcon
            v-if="member.role === 'operator'"
            name="shield"
            :size="18"
            class="text-success"
          />
        </div>
        <p class="profile-nick flex items-center gap-2.5 text-[13px] text-muted-foreground">
          @{{ member.nick
          }}<span class="flex items-center gap-[5px] capitalize"
            ><i
              class="status-dot inline-block w-[7px] h-[7px] bg-success rounded-full shrink-0"
              :class="{
                'is-offline': member.status === 'offline',
                'is-away': member.status === 'away',
              }"
            />{{ member.status }}</span
          ><span
            v-if="member.status === 'away' && member.awayMessage"
            class="profile-away truncate"
            >{{ member.awayMessage }}</span
          >
        </p>
        <!-- A #channel button opens the channel behind the dialog, so the dialog steps aside. -->
        <p
          class="profile-bio mt-3.5 p-3 rounded-md bg-card text-[14px] text-body leading-[1.55] whitespace-pre-wrap wrap-anywhere"
          @click.capture="closeOnChannel"
        >
          <MessageRuns :text="member.bio ?? ''" />
        </p>
        <dl
          class="profile-details grid [grid-template-columns:repeat(auto-fit,_minmax(120px,_1fr))] gap-3 mt-3.5 mr-0 mb-4.5 ml-0 [&_dd]:mt-0.5 [&_dd]:text-[13px] [&_dt]:text-[11px] [&_dt]:font-[600] [&_dt]:uppercase [&_dt]:tracking-[0.05em] [&_dt]:text-faint"
        >
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
        <UiAlert
          as="p"
          variant="destructive"
          presentation="inline"
          v-if="error"
          class="form-error mt-3"
        >
          {{ error }}
        </UiAlert>
        <UiButton
          v-if="member.id === 'you'"
          class="w-full"
          @click="emit('edit')"
        >
          <AppIcon
            name="settings"
            :size="17"
          />Edit profile</UiButton
        ><UiButton
          v-else
          class="w-full"
          :disabled="chat.direct.isLoading"
          @click="message"
        >
          <AppIcon
            name="message"
            :size="17"
          />{{ chat.direct.isLoading ? 'Opening…' : 'Message' }}
        </UiButton>
      </div>
    </div></AppDialog
  >
</template>
