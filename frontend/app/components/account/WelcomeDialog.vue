<script setup vapor lang="ts">
import type { Member } from '../../../shared/types/chat'
import { explicitDisplayName, fallbackDisplayName } from '../../../shared/utils/identity'
import { nicknameError, nicknameLimit } from '../../../shared/utils/nickname'
import {
  DEFAULT_AVATAR_PATH,
  defaultAvatarFor,
  defaultAvatarUrl,
  suggestAvatar,
  type DefaultAvatar,
} from '../../../shared/utils/default-avatars'
import {
  guideHasChoices,
  guideKey,
  initialGuideSelection,
} from '../../../shared/utils/channel-guide'
/**
 * One-time welcome setup for a newly registered account arriving on its provisioned default
 * IRCord server. Step one keeps the profile and IRC identity on that server; step two picks
 * channels from the server's channel guide (#258) when it offers any. Onboarding completes
 * only after all requested saves succeed.
 */
const props = defineProps<{ networkId: string }>()
const emit = defineEmits<{ close: [] }>()

const auth = useAuthStore()
const chat = useChatStore()
const { t, n, locale } = useI18n()
const $t = t

const user = computed(() => chat.selfFor(props.networkId))
const network = computed(() => chat.data?.networks.find((item) => item.id === props.networkId))
const networkName = computed(() => network.value?.name || 'IRCord')

const savedNick = ref(network.value?.nick || auth.account?.username || '')
const savedRealname = ref(network.value?.realname ?? '')
const savedName = ref(explicitDisplayName(user.value))
const savedBio = ref(user.value?.bio ?? '')
const savedAvatarUrl = ref(user.value?.avatarUrl ?? '')
const profileReady = ref(!!network.value?.profile)
const loadingProfile = ref(false)

const nick = ref(savedNick.value)
const realname = ref(savedRealname.value)
const name = ref(savedName.value)
const bio = ref(savedBio.value)

type AvatarChoice =
  | { kind: 'default'; avatar: DefaultAvatar }
  | { kind: 'upload'; dataUrl: string }
  | { kind: 'none' }
const pickedAvatar = ref<AvatarChoice>()
const { avatars } = useDefaultAvatars()
const suggestion = computed(() =>
  suggestAvatar(nick.value.trim() || auth.account?.username || '', avatars.value),
)
const selectedDefault = computed(() =>
  pickedAvatar.value?.kind === 'default'
    ? pickedAvatar.value.avatar.id
    : pickedAvatar.value === undefined
      ? defaultAvatarFor(savedAvatarUrl.value || undefined, avatars.value, location.origin)?.id
      : undefined,
)
const previewAvatarUrl = computed(() => {
  if (pickedAvatar.value?.kind === 'default')
    return DEFAULT_AVATAR_PATH + pickedAvatar.value.avatar.file
  if (pickedAvatar.value?.kind === 'upload') return pickedAvatar.value.dataUrl
  if (pickedAvatar.value?.kind === 'none') return undefined
  return savedAvatarUrl.value || undefined
})
const fallbackName = computed(() =>
  fallbackDisplayName(
    user.value,
    nick.value.trim() || savedNick.value,
    realname.value.trim() || savedRealname.value || undefined,
  ),
)
const previewMember = computed<Member>(() => ({
  ...(user.value ?? {
    id: 'you',
    nick: savedNick.value,
    displayName: savedNick.value,
    account: '',
    bio: '',
    color: '',
    role: 'member',
    status: 'online',
  }),
  nick: nick.value.trim() || savedNick.value,
  displayName: name.value.trim() || fallbackName.value || savedNick.value || '?',
  avatarUrl: previewAvatarUrl.value,
}))

const nickMax = computed(() => nicknameLimit(network.value?.isupport))
const nickProblem = computed(() =>
  nicknameError(nick.value.trim(), nickMax.value, (key, values) => t(key, values ?? {})),
)
const nickTouched = ref(false)
const nickError = computed(() =>
  nickTouched.value || nick.value.trim().length > 0 ? nickProblem.value : '',
)
const nickInput = ref<{
  $el?: HTMLInputElement
  focus: () => void
  setCustomValidity: (message: string) => void
} | null>(null)
watchEffect(() => nickInput.value?.setCustomValidity(nickProblem.value))

const avatarInput = ref<HTMLInputElement | null>(null)
const processingAvatar = ref(false)
const saving = ref(false)
const skipping = ref(false)
const completed = ref(false)
const error = ref('')
const channelGuide = useChannelGuide(() => props.networkId)
const preferences = usePreferencesStore()
const busy = computed(
  () =>
    saving.value ||
    skipping.value ||
    channelGuide.joining.value ||
    auth.completeOnboarding.isLoading,
)

// Step two: the channel guide. Channels for everyone were joined at signup; suggested ones
// start ticked. The step is left out when the guide has nothing to choose.
const step = ref<'profile' | 'channels'>('profile')
const guideEntries = computed(() =>
  channelGuide.guide.data.value?.available ? channelGuide.guide.data.value.entries : [],
)
const hasChoices = computed(() => guideHasChoices(guideEntries.value, channelGuide.joined.value))
const preselected = computed(() =>
  initialGuideSelection(guideEntries.value, channelGuide.joined.value),
)
const picked = ref<string[]>([])
watch(
  () => channelGuide.guide.data.value,
  (value) => {
    if (value && step.value === 'profile') picked.value = preselected.value
  },
  { immediate: true },
)
const joinFailedBefore = ref(false)
/** The guide couldn't be loaded; the channel step offers a retry instead of a list. */
const guideFailed = computed(
  () => !!channelGuide.guide.error.value && !channelGuide.guide.data.value,
)
const retryingGuide = ref(false)
async function retryGuide() {
  retryingGuide.value = true
  try {
    await channelGuide.guide.refetch()
    if (channelGuide.guide.data.value) picked.value = preselected.value
  } finally {
    retryingGuide.value = false
  }
}

/** Joins the chosen channels, completes onboarding and opens the first channel for everyone. */
async function finish(channels: string[]) {
  error.value = ''
  if (channels.length) {
    const result = await channelGuide.join(channels)
    // The first time some channels can't be joined, stay on the channel step with them ticked,
    // so you see it and can try again. After that, finishing goes ahead without them.
    if (result.failed.length && !joinFailedBefore.value) {
      joinFailedBefore.value = true
      step.value = 'channels'
      picked.value = result.failed.map((f) => guideKey(f.channel))
      error.value = t('welcome.joinFailed', {
        channels: result.failed.map((f) => f.channel).join(', '),
      })
      return
    }
  }
  await auth.completeOnboarding.mutateAsync()
  completed.value = true
  // You just saw the guide; the notice for earlier members isn't for you.
  if (!preferences.preferences.channelGuideSeen) preferences.update({ channelGuideSeen: true })
  const first = guideEntries.value.find((c) => c.state === 'everyone')
  const landing =
    first &&
    chat.data?.conversations.find(
      (c) =>
        c.networkId === props.networkId &&
        c.kind === 'channel' &&
        guideKey(c.target || `#${c.name}`) === guideKey(first.channel),
    )
  if (landing) chat.activate(landing.id)
  emit('close')
}

/**
 * After the profile step: show the channel step, or finish when the guide offers nothing to
 * pick. A guide still loading is waited for; one that can't be loaded shows the channel step
 * with a way to retry, so the step is never skipped by accident.
 */
async function next() {
  if (!channelGuide.guide.data.value) await channelGuide.guide.refetch()
  if (guideFailed.value || hasChoices.value) {
    step.value = 'channels'
    return
  }
  await finish([])
}

async function joinPicked() {
  if (busy.value) return
  saving.value = true
  try {
    await finish(picked.value)
  } catch (e) {
    error.value = errorText(e, (key) => t(key))
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  if (profileReady.value) return
  loadingProfile.value = true
  try {
    const member = await chat.loadNetworkProfile(props.networkId)
    savedName.value = explicitDisplayName(member)
    savedBio.value = member.bio ?? ''
    savedAvatarUrl.value = member.avatarUrl ?? ''
    name.value = savedName.value
    bio.value = savedBio.value
    profileReady.value = true
  } catch (cause) {
    error.value = errorText(cause, (key) => t(key))
  } finally {
    loadingProfile.value = false
  }
})

const profileDirty = computed(
  () =>
    name.value.trim() !== savedName.value ||
    bio.value !== savedBio.value ||
    pickedAvatar.value !== undefined,
)
const networkDirty = computed(
  () =>
    !!network.value &&
    (nick.value.trim() !== savedNick.value || realname.value.trim() !== savedRealname.value),
)

async function chooseAvatar(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  error.value = ''
  processingAvatar.value = true
  try {
    pickedAvatar.value = {
      kind: 'upload',
      dataUrl: await prepareAvatarImage(file, (key, values) => t(key, values ?? {}), locale.value),
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('profile.imageOpenFailed')
  } finally {
    processingAvatar.value = false
    if (avatarInput.value) avatarInput.value.value = ''
  }
}

function chooseDefault(avatar: DefaultAvatar) {
  error.value = ''
  pickedAvatar.value = { kind: 'default', avatar }
}

function removeAvatar() {
  error.value = ''
  pickedAvatar.value = savedAvatarUrl.value ? { kind: 'none' } : undefined
}

function onInvalid(event: Event) {
  event.preventDefault()
  nickTouched.value = true
  nickInput.value?.focus()
}

async function save() {
  if (busy.value || processingAvatar.value || !profileReady.value) return
  nickTouched.value = true
  if (nickProblem.value) {
    nickInput.value?.focus()
    return
  }
  saving.value = true
  error.value = ''
  try {
    if (profileDirty.value) {
      const nameChanged = name.value.trim() !== savedName.value
      const avatarPatch =
        pickedAvatar.value?.kind === 'default'
          ? { avatarUrl: defaultAvatarUrl(pickedAvatar.value.avatar, location.origin) }
          : pickedAvatar.value?.kind === 'upload'
            ? { avatarDataUrl: pickedAvatar.value.dataUrl }
            : pickedAvatar.value?.kind === 'none'
              ? { avatarDataUrl: null }
              : {}
      const updatedMember = await chat.networkProfile.mutateAsync({
        id: props.networkId,
        input: {
          ...(nameChanged ? { displayName: name.value.trim() } : {}),
          bio: bio.value,
          ...avatarPatch,
        },
      })
      savedName.value = explicitDisplayName(updatedMember)
      name.value = savedName.value
      savedBio.value = updatedMember.bio ?? ''
      savedAvatarUrl.value = updatedMember.avatarUrl ?? ''
      pickedAvatar.value = undefined
    }
    if (networkDirty.value && network.value) {
      const trimmedNick = nick.value.trim()
      const trimmedRealname = realname.value.trim()
      const nickChanged = trimmedNick !== savedNick.value
      const realnameChanged = trimmedRealname !== savedRealname.value
      const updatedNetwork = await chat.updateNetwork.mutateAsync({
        id: network.value.id,
        input: {
          ...(nickChanged ? { nick: trimmedNick } : {}),
          ...(realnameChanged ? { realname: trimmedRealname } : {}),
        },
      })
      savedNick.value = updatedNetwork.nick
      nick.value = updatedNetwork.nick
      savedRealname.value = updatedNetwork.realname ?? ''
      realname.value = updatedNetwork.realname ?? ''
    }
    await next()
  } catch (e) {
    error.value = errorText(e, (key) => t(key))
  } finally {
    saving.value = false
  }
}

/**
 * Skipping the profile moves on to the channels. Skipping the channels joins the ones that
 * started ticked, as joining the default channels did before there was a choice.
 */
async function skip() {
  if (busy.value) return
  skipping.value = true
  error.value = ''
  try {
    if (step.value === 'profile') await next()
    else await finish(preselected.value)
  } catch (e) {
    error.value = errorText(e, (key) => t(key))
  } finally {
    skipping.value = false
  }
}

/** Closing the dialog skips the rest of the setup. */
async function dismiss() {
  if (busy.value) return
  skipping.value = true
  error.value = ''
  try {
    await finish(hasChoices.value ? preselected.value : [])
  } catch (e) {
    error.value = errorText(e, (key) => t(key))
  } finally {
    skipping.value = false
  }
}

/**
 * Closing the dialog yourself skips the rest of the setup. Navigation (such as the startup
 * redirect to your first channel) only closes it; the workspace opens it again afterwards.
 */
function onBeforeClose(byUser: boolean) {
  if (completed.value || !byUser) return true
  void dismiss()
  return false
}
</script>
<template>
  <AppDialog
    :title="step === 'profile' ? $t('welcome.title') : $t('welcome.channelsTitle')"
    :wide="step === 'channels'"
    :before-close="onBeforeClose"
    @close="emit('close')"
  >
    <form
      v-if="step === 'channels'"
      class="welcome-dialog welcome-channels grid gap-3.5 px-5 pb-5 text-[13px]"
      @submit.prevent="joinPicked"
    >
      <p class="m-0 text-[13px] leading-[1.5] text-muted-foreground">
        {{ $t('welcome.channelsIntro', { network: networkName }) }}
      </p>
      <UiAlert
        v-if="guideFailed"
        as="div"
        variant="destructive"
        presentation="inline"
        class="guide-load-error"
      >
        <p class="m-0">{{ $t('welcome.guideFailed') }}</p>
        <UiButton
          type="button"
          variant="link"
          size="text"
          :disabled="retryingGuide"
          @click="retryGuide"
          >{{ $t('welcome.retryGuide') }}</UiButton
        >
      </UiAlert>
      <div
        v-else
        class="overflow-y-auto max-h-[min(440px,_55dvh)] -mx-2"
      >
        <ChannelGuidePicker
          v-model="picked"
          :groups="channelGuide.guide.data.value?.groups ?? []"
          :channels="guideEntries"
          :joined="channelGuide.joined.value"
          :disabled="busy"
        />
      </div>
      <UiAlert
        v-if="error"
        as="p"
        variant="destructive"
        presentation="inline"
        class="form-error"
      >
        {{ error }}
      </UiAlert>
      <div class="form-actions flex items-center justify-end gap-2 pt-1">
        <UiButton
          type="button"
          variant="ghost"
          :disabled="busy"
          @click="skip"
        >
          {{ skipping ? $t('welcome.skipping') : $t('welcome.skipChannels') }}
        </UiButton>
        <UiButton
          type="submit"
          :disabled="busy"
        >
          {{
            saving
              ? $t('channelGuide.joining')
              : picked.length
                ? $t('channelGuide.joinCount', { count: n(picked.length) }, picked.length)
                : $t('welcome.continue')
          }}
        </UiButton>
      </div>
    </form>
    <form
      v-else
      class="welcome-dialog grid gap-3.5 px-5 pb-5 text-[13px]"
      @submit.prevent="save"
      @invalid.capture="onInvalid"
    >
      <p class="m-0 text-[13px] leading-[1.5] text-muted-foreground">
        {{ $t('welcome.intro', { network: networkName }) }}
      </p>

      <div class="welcome-identity grid grid-cols-[auto_1fr] items-center gap-3.5">
        <button
          type="button"
          class="welcome-avatar-button group relative p-0 border-0 rounded-full [background:none] cursor-pointer"
          :aria-label="previewMember.avatarUrl ? $t('profile.changeImage') : $t('profile.addImage')"
          :title="$t('profile.imageLimits')"
          :disabled="processingAvatar || busy"
          @click="avatarInput?.click()"
        >
          <UserAvatar
            :member="previewMember"
            size="large"
          /><span
            class="welcome-avatar-overlay absolute inset-0 grid [place-items:center] rounded-full [background:color-mix(in_srgb,_#000_45%,_transparent)] text-[#fff] opacity-0 [transition:opacity_0.15s] group-hover:opacity-100 group-focus-visible:opacity-100"
            aria-hidden="true"
            ><AppIcon
              name="upload"
              :size="18"
          /></span>
        </button>
        <div class="grid gap-1 min-w-0">
          <div class="flex items-center gap-2 min-w-0">
            <strong class="truncate text-[14px] font-[650] text-foreground">{{
              previewMember.displayName
            }}</strong>
            <span class="truncate text-[12px] text-faint">@{{ previewMember.nick }}</span>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <UiButton
              type="button"
              variant="secondary"
              size="sm"
              :disabled="processingAvatar || busy"
              @click="avatarInput?.click()"
            >
              <AppIcon
                name="upload"
                :size="14"
              />{{ processingAvatar ? $t('profile.preparing') : $t('login.uploadOwn') }}
            </UiButton>
            <UiButton
              v-if="previewMember.avatarUrl"
              type="button"
              variant="link"
              size="text"
              :disabled="processingAvatar || busy"
              @click="removeAvatar"
            >
              {{ $t('common.remove') }}
            </UiButton>
          </div>
          <input
            ref="avatarInput"
            class="sr-only"
            type="file"
            accept="image/png,image/jpeg,image/webp"
            :aria-label="$t('profile.uploadImage')"
            tabindex="-1"
            :disabled="processingAvatar || busy"
            @change="chooseAvatar"
          />
        </div>
      </div>

      <DefaultAvatarPicker
        v-if="avatars.length"
        :avatars="avatars"
        :selected="selectedDefault"
        :suggested="!pickedAvatar && !savedAvatarUrl ? suggestion?.id : undefined"
        name="welcome-avatar"
        :label="$t('profile.defaultPicturesLabel')"
        :disabled="processingAvatar || busy"
        class="max-h-28"
        @select="chooseDefault"
      />

      <AppField
        id="welcome-nick"
        :label="$t('networkForm.nickname')"
        :error="nickError"
        :help="nickError ? undefined : $t('welcome.nicknameHelp', { network: networkName })"
      >
        <template #default="control">
          <UiInput
            v-bind="control"
            ref="nickInput"
            v-model="nick"
            autofocus
            :maxlength="nickMax"
            autocomplete="off"
            autocapitalize="none"
            spellcheck="false"
            required
          />
        </template>
      </AppField>

      <AppInput
        id="welcome-display-name"
        v-model="name"
        :placeholder="fallbackName"
        :help="$t('welcome.displayNameHelp')"
        maxlength="40"
        autocomplete="nickname"
      >
        <template #label
          >{{ $t('profile.displayName') }} <small>{{ $t('common.optional') }}</small></template
        >
      </AppInput>

      <AppInput
        id="welcome-realname"
        v-model="realname"
        :help="$t('welcome.realNameHelp', { network: networkName })"
        maxlength="100"
        autocomplete="off"
      >
        <template #label
          >{{ $t('networkForm.realName') }} <small>{{ $t('common.optional') }}</small></template
        >
      </AppInput>

      <div class="grid gap-1.5">
        <div class="flex items-baseline justify-between">
          <FieldLabel
            for="welcome-bio"
            class="m-0"
            >{{ $t('profile.aboutYou') }} <small>{{ $t('common.optional') }}</small></FieldLabel
          >
          <FieldHint
            as="small"
            aria-live="polite"
            >{{ n(bio.length) }}/{{ n(300) }}</FieldHint
          >
        </div>
        <UiTextarea
          id="welcome-bio"
          v-model="bio"
          rows="2"
          maxlength="300"
          :placeholder="$t('profile.bioPlaceholder')"
        />
      </div>

      <UiAlert
        v-if="error"
        as="p"
        variant="destructive"
        presentation="inline"
        class="form-error"
      >
        {{ error }}
      </UiAlert>

      <div class="form-actions flex items-center justify-end gap-2 pt-1">
        <UiButton
          type="button"
          variant="ghost"
          :disabled="busy"
          @click="skip"
        >
          {{ skipping ? $t('welcome.skipping') : $t('welcome.skip') }}
        </UiButton>
        <UiButton
          type="submit"
          :disabled="busy || processingAvatar || loadingProfile || !profileReady"
        >
          {{ saving ? $t('common.saving') : hasChoices ? $t('welcome.next') : $t('welcome.save') }}
        </UiButton>
      </div>
    </form>
  </AppDialog>
</template>
