<script setup lang="ts">
import {
  explicitDisplayName,
  fallbackDisplayName,
  memberNameColor,
} from '../../shared/utils/identity'
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
      avatarDataUrl.value !== undefined),
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
        avatarUrl:
          avatarDataUrl.value === undefined
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
    if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type))
      throw new Error('Choose a PNG, JPEG or WebP image.')
    if (file.size > 5 * 1024 * 1024) throw new Error('Choose an image smaller than 5 MB.')
    const bitmap = await createImageBitmap(file)
    try {
      const canvas = document.createElement('canvas')
      canvas.width = canvas.height = 256
      const context = canvas.getContext('2d')
      if (!context) throw new Error('Your browser could not prepare this image.')
      const side = Math.min(bitmap.width, bitmap.height)
      context.drawImage(
        bitmap,
        (bitmap.width - side) / 2,
        (bitmap.height - side) / 2,
        side,
        side,
        0,
        0,
        256,
        256,
      )
      avatarDataUrl.value = canvas.toDataURL('image/webp', 0.85)
    } finally {
      bitmap.close()
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'This image could not be opened.'
  } finally {
    processingAvatar.value = false
    if (avatarInput.value) avatarInput.value.value = ''
  }
}
function removeAvatar() {
  avatarDataUrl.value = null
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
    })
    avatarDataUrl.value = undefined
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
      <h4
        class="settings-subtitle text-[12px] font-[650] uppercase tracking-[0.05em] text-faint pb-1.5 border-b [border-bottom-style:solid] border-b-border"
      >
        Your profile
      </h4>
      <p class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5">
        How you appear in IRCord. {{ sharing }}
      </p>
      <div class="profile-identity flex items-start gap-5 mt-4 mr-0 mb-5 ml-0">
        <div
          data-setting="profile-image"
          class="profile-avatar flex flex-col items-center gap-1.5 flex-none"
        >
          <button
            type="button"
            class="profile-avatar-button relative p-0 border-0 rounded-full [background:none] cursor-pointer"
            :aria-label="previewMember?.avatarUrl ? 'Change profile image' : 'Add a profile image'"
            title="PNG, JPEG or WebP, up to 5 MB"
            :disabled="processingAvatar || chat.profile.isLoading"
            @click="avatarInput?.click()"
          >
            <UserAvatar
              :member="previewMember"
              size="large"
            /><span
              class="profile-avatar-overlay absolute inset-0 grid [place-items:center] rounded-full [background:color-mix(in_srgb,_#000_45%,_transparent)] text-[#fff] [opacity:0] [transition:opacity_0.15s]"
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
          />
          <UiButton
            variant="link"
            size="text"
            v-if="previewMember?.avatarUrl"
            type="button"
            aria-label="Remove image"
            :disabled="processingAvatar || chat.profile.isLoading"
            @click="removeAvatar"
          >
            Remove
          </UiButton>
          <small v-else-if="processingAvatar">Preparing…</small>
        </div>
        <div
          data-setting="display-name"
          class="profile-name flex-1 min-w-0"
        >
          <UiLabel
            class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
            for="profile-name"
            >Display name</UiLabel
          ><UiInput
            id="profile-name"
            v-model="name"
            maxlength="40"
            :placeholder="fallbackName"
            aria-describedby="profile-name-hint"
          />
          <p
            id="profile-name-hint"
            class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
          >
            Shown as
            <strong :style="nameStyle(previewMember)">{{ previewMember?.displayName }}</strong>
            · @{{ chat.network?.nick ?? user?.nick
            }}<template v-if="!name.trim()"> · Leave blank to use your IRC name</template>
          </p>
        </div>
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
            type="color"
            aria-label="Custom name color"
            :disabled="colorMode !== 'custom'"
          />
        </UiRadioGroup>
      </div>
      <div
        data-setting="about-you"
        class="profile-about flex justify-between items-baseline mt-5 mr-0 mb-1.5 ml-0"
      >
        <UiLabel
          class="field-label block text-[12px] font-[600] text-muted-foreground mt-4 mr-0 mb-1.5 ml-0"
          for="profile-bio"
          >About you</UiLabel
        ><small
          class="field-hint text-[12px] text-faint block leading-[1.5] mt-1.5"
          aria-live="polite"
          >{{ bio.length }}/300</small
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
          as="p"
          variant="destructive"
          presentation="inline"
          v-if="error"
          class="form-error mt-3"
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
