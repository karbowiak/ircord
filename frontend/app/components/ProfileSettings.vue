<script setup lang="ts">
import {
  explicitDisplayName,
  fallbackDisplayName,
  memberNameColor,
} from '../../shared/utils/identity'
import {
  DEFAULT_AVATAR_PATH,
  defaultAvatarFor,
  defaultAvatarUrl,
  type DefaultAvatar,
} from '../../shared/utils/default-avatars'
/**
 * Settings → Profile. First how you appear in IRCord (account-wide: picture, display name,
 * color, about), then your IRC identity per network (nickname, real name, away).
 */
const chat = useChatStore()
const appearance = useAppearanceStore()
const nameStyle = useNameStyle()
const error = ref('')
const saved = ref(false)
const user = computed(() => chat.data?.user)
/** Only a name the user chose; blank means the network nickname or real name shows. */
const explicitName = computed(() => explicitDisplayName(user.value))
const name = ref(explicitName.value)
/** What others see while no profile name is set (also while clearing a saved one). */
const fallbackName = computed(() =>
  fallbackDisplayName(user.value, chat.network?.nick, chat.network?.realname),
)
const bio = ref(user.value?.bio ?? '')
const colorMode = ref(user.value?.nameColor ? 'custom' : 'automatic')
const customColor = ref(
  user.value?.nameColor || memberNameColor(user.value, appearance.mode, appearance.tokens.main),
)
const avatarDataUrl = ref<string | null>()
/** A default avatar picked here and not saved yet. */
const defaultChoice = ref<DefaultAvatar>()
const { avatars } = useDefaultAvatars()
/** The default shown as chosen: the one just picked, else the saved one if it is a default. */
const selectedDefault = computed(() =>
  defaultChoice.value
    ? defaultChoice.value.id
    : avatarDataUrl.value === undefined
      ? defaultAvatarFor(user.value?.avatarUrl, avatars.value, location.origin)?.id
      : undefined,
)
watch([name, bio, colorMode, customColor], () => {
  saved.value = false
})
/** Save is offered only when something differs from what's stored. */
const dirty = computed(
  () =>
    !!user.value &&
    (name.value.trim() !== explicitName.value ||
      bio.value !== (user.value.bio ?? '') ||
      (colorMode.value === 'custom' ? customColor.value : null) !==
        (user.value.nameColor ?? null) ||
      avatarDataUrl.value !== undefined ||
      defaultChoice.value !== undefined),
)
/** Networks that carry IRCord profiles (IRCv3 metadata); elsewhere others see your real name. */
const sharing = computed(() => {
  const networks = chat.data?.networks ?? []
  const shared = networks.filter((n) => n.features.metadata?.includes('display-name'))
  if (shared.length === networks.length) return 'Other IRCord users see it too.'
  if (!shared.length)
    return 'Your networks can’t share profiles, so other IRCord users see your IRC real name.'
  return `Shared with other IRCord users on ${shared.map((n) => n.name).join(', ')}; elsewhere they see your IRC real name.`
})
const avatarInput = ref<HTMLInputElement | null>(null)
const processingAvatar = ref(false)
const previewMember = computed(() =>
  user.value
    ? {
        ...user.value,
        displayName: name.value.trim() || fallbackName.value,
        nameColor: colorMode.value === 'custom' ? customColor.value : undefined,
        avatarUrl: defaultChoice.value
          ? DEFAULT_AVATAR_PATH + defaultChoice.value.file
          : avatarDataUrl.value === undefined
            ? user.value.avatarUrl
            : avatarDataUrl.value || undefined,
      }
    : undefined,
)
async function chooseAvatar(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  error.value = ''
  saved.value = false
  processingAvatar.value = true
  try {
    avatarDataUrl.value = await prepareAvatarImage(file)
    defaultChoice.value = undefined
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'This image could not be opened.'
  } finally {
    processingAvatar.value = false
    if (avatarInput.value) avatarInput.value.value = ''
  }
}
function chooseDefault(avatar: DefaultAvatar) {
  defaultChoice.value = avatar
  avatarDataUrl.value = undefined
  error.value = ''
  saved.value = false
}
function removeAvatar() {
  avatarDataUrl.value = null
  defaultChoice.value = undefined
  saved.value = false
}
async function saveProfile() {
  saved.value = false
  error.value = ''
  try {
    const nameChanged = name.value.trim() !== explicitName.value
    await chat.profile.mutateAsync({
      // Send the name only when it changed, so saving anything else never turns the
      // nickname fallback into a chosen name. An empty name clears the override.
      ...(nameChanged ? { displayName: name.value.trim() } : {}),
      bio: bio.value,
      nameColor: colorMode.value === 'custom' ? customColor.value : null,
      ...(avatarDataUrl.value !== undefined ? { avatarDataUrl: avatarDataUrl.value } : {}),
      ...(defaultChoice.value
        ? { avatarUrl: defaultAvatarUrl(defaultChoice.value, location.origin) }
        : {}),
    })
    avatarDataUrl.value = undefined
    defaultChoice.value = undefined
    if (name.value !== explicitName.value) {
      name.value = explicitName.value // The server trims; show what was stored.
      await nextTick()
    }
    saved.value = true
  } catch (e) {
    error.value = errorText(e)
  }
}
</script>
<template>
  <div class="profile-settings">
    <form
      class="profile-form"
      @submit.prevent="saveProfile"
    >
      <SettingsSubtitle> Your profile </SettingsSubtitle>
      <FieldHint> How you appear in IRCord. {{ sharing }} </FieldHint>
      <div class="profile-identity flex items-start gap-5 mt-4 mr-0 mb-5 ml-0">
        <div
          data-setting="profile-image"
          class="profile-avatar flex flex-col items-center gap-1.5 flex-none"
        >
          <button
            type="button"
            class="profile-avatar-button group relative p-0 border-0 rounded-full [background:none] cursor-pointer"
            :aria-label="previewMember?.avatarUrl ? 'Change profile image' : 'Add a profile image'"
            title="PNG, JPEG or WebP, up to 5 MB"
            :disabled="processingAvatar || chat.profile.isLoading"
            @click="avatarInput?.click()"
          >
            <UserAvatar
              :member="previewMember"
              size="large"
            /><span
              class="profile-avatar-overlay absolute inset-0 grid [place-items:center] rounded-full [background:color-mix(in_srgb,_#000_45%,_transparent)] text-[#fff] opacity-0 [transition:opacity_0.15s] group-hover:opacity-100 group-focus-visible:opacity-100"
              aria-hidden="true"
              ><AppIcon
                name="upload"
                :size="18"
            /></span>
          </button>
          <input
            ref="avatarInput"
            class="sr-only"
            type="file"
            accept="image/png,image/jpeg,image/webp"
            aria-label="Upload profile image"
            tabindex="-1"
            :disabled="processingAvatar || chat.profile.isLoading"
            @change="chooseAvatar"
          >
          <UiButton
            v-if="previewMember?.avatarUrl"
            variant="link"
            size="text"
            type="button"
            aria-label="Remove image"
            :disabled="processingAvatar || chat.profile.isLoading"
            @click="removeAvatar"
          >
            Remove
          </UiButton>
          <small
            v-else-if="processingAvatar"
            class="text-[12px] text-muted-foreground"
            >Preparing…</small
          >
        </div>
        <div
          data-setting="display-name"
          class="profile-name flex-1 min-w-0"
        >
          <AppField
            id="profile-name"
            label="Display name"
          >
            <template #default="control">
              <UiInput
                v-bind="control"
                v-model="name"
                maxlength="40"
                :placeholder="fallbackName"
              />
            </template>
            <template #help>
              Shown as
              <strong
                class="font-[650]"
                :style="nameStyle(previewMember)"
                >{{ previewMember?.displayName }}</strong
              >
              · @{{ chat.network?.nick ?? user?.nick
              }}<template v-if="!name.trim()"> · Leave blank to use your IRC name</template>
            </template>
          </AppField>
        </div>
      </div>
      <div
        v-if="avatars.length"
        data-setting="default-avatars"
        class="profile-defaults grid gap-2 mb-5"
      >
        <span class="setting-copy flex flex-col gap-px"
          ><strong>Default pictures</strong
          ><small
            >Pick one, or
            <UiButton
              type="button"
              variant="inline"
              size="inline"
              :disabled="processingAvatar || chat.profile.isLoading"
              @click="avatarInput?.click()"
            >
              upload your own image
            </UiButton></small
          ></span
        >
        <DefaultAvatarPicker
          :avatars="avatars"
          :selected="selectedDefault"
          name="profile-avatar"
          label="Default profile pictures"
          :disabled="processingAvatar || chat.profile.isLoading"
          @select="chooseDefault"
        />
      </div>
      <div
        data-setting="name-colour"
        class="setting-row flex items-center justify-between gap-4"
      >
        <span class="setting-copy flex flex-col gap-[1px] flex-1 min-w-0"
          ><strong id="profile-color-label">Name color</strong
          ><small>Adjusted for readability in every theme.</small></span
        >
        <UiRadioGroup
          class="name-color-options flex items-center gap-4 text-[13px]"
          :model-value="colorMode"
          name="name-color"
          aria-labelledby="profile-color-label"
          @update:model-value="colorMode = String($event)"
        >
          <UiLabel
            for="profile-color-automatic"
            class="inline-flex items-center gap-1.5 text-[13px] font-normal text-foreground"
            ><UiRadioGroupItem
              id="profile-color-automatic"
              value="automatic"
            />Automatic</UiLabel
          ><UiLabel
            for="profile-color-custom"
            class="inline-flex items-center gap-1.5 text-[13px] font-normal text-foreground"
            ><UiRadioGroupItem
              id="profile-color-custom"
              value="custom"
            />Custom</UiLabel
          ><input
            v-model="customColor"
            class="h-[26px] w-9 rounded-sm border border-solid border-border bg-transparent p-0"
            type="color"
            aria-label="Custom name color"
            :disabled="colorMode !== 'custom'"
          >
        </UiRadioGroup>
      </div>
      <div
        data-setting="about-you"
        class="profile-about flex justify-between items-baseline mt-5 mr-0 mb-1.5 ml-0"
      >
        <FieldLabel
          for="profile-bio"
          class="m-0"
          >About you</FieldLabel
        ><FieldHint
          as="small"
          aria-live="polite"
          >{{ bio.length }}/300</FieldHint
        >
      </div>
      <UiTextarea
        id="profile-bio"
        v-model="bio"
        rows="3"
        maxlength="300"
        placeholder="A line or two people see on your profile"
      />
      <div class="form-actions flex justify-end items-center gap-2 mt-5">
        <UiAlert
          v-if="error"
          as="p"
          variant="destructive"
          presentation="inline"
          class="form-error mr-auto"
        >
          {{ error }}
        </UiAlert>
        <span
          v-else-if="saved"
          class="success-text text-success flex gap-[5px] items-center text-[13px] mr-auto"
          role="status"
          ><AppIcon
            name="check"
            :size="15"
          />Profile saved</span
        ><UiButton :disabled="!dirty || chat.profile.isLoading || processingAvatar">
          {{ chat.profile.isLoading ? 'Saving…' : 'Save changes' }}
        </UiButton>
      </div>
    </form>
    <NetworkIdentity />
  </div>
</template>
