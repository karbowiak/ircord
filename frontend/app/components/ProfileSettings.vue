<script setup lang="ts">
import { memberNameColor } from '../../shared/utils/identity'
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
const name = ref(user.value?.displayName ?? '')
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
    (name.value !== user.value.displayName ||
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
        displayName: name.value || user.value.displayName,
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
    await chat.profile.mutateAsync({
      displayName: name.value,
      bio: bio.value,
      nameColor: colorMode.value === 'custom' ? customColor.value : null,
      ...(avatarDataUrl.value !== undefined ? { avatarDataUrl: avatarDataUrl.value } : {}),
    })
    avatarDataUrl.value = undefined
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
      <h4 class="settings-subtitle">Your profile</h4>
      <p class="field-hint">How you appear in IRCord. {{ sharing }}</p>
      <div class="profile-identity">
        <div class="profile-avatar">
          <button
            type="button"
            class="profile-avatar-button"
            :aria-label="previewMember?.avatarUrl ? 'Change profile image' : 'Add a profile image'"
            title="PNG, JPEG or WebP, up to 5 MB"
            :disabled="processingAvatar || chat.profile.isLoading"
            @click="avatarInput?.click()"
          >
            <UserAvatar
              :member="previewMember"
              size="large"
            /><span
              class="profile-avatar-overlay"
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
          <button
            v-if="previewMember?.avatarUrl"
            type="button"
            class="text-button"
            aria-label="Remove image"
            :disabled="processingAvatar || chat.profile.isLoading"
            @click="removeAvatar"
          >
            Remove
          </button>
          <small v-else-if="processingAvatar">Preparing…</small>
        </div>
        <div class="profile-name">
          <label
            class="field-label"
            for="profile-name"
            >Display name</label
          ><input
            id="profile-name"
            v-model="name"
            class="text-input"
            maxlength="40"
            required
          />
          <p class="field-hint">
            Shown as
            <strong :style="nameStyle(previewMember)">{{ name || 'Your name' }}</strong>
            · @{{ chat.network?.nick ?? user?.nick }}
          </p>
        </div>
      </div>
      <div class="setting-row">
        <span class="setting-copy"
          ><strong id="profile-color-label">Name color</strong
          ><small>Adjusted for readability in every theme.</small></span
        >
        <div
          class="name-color-options"
          role="radiogroup"
          aria-labelledby="profile-color-label"
        >
          <label
            ><input
              v-model="colorMode"
              type="radio"
              value="automatic"
              name="name-color"
            />Automatic</label
          ><label
            ><input
              v-model="colorMode"
              type="radio"
              value="custom"
              name="name-color"
            />Custom</label
          ><input
            v-model="customColor"
            type="color"
            aria-label="Custom name color"
            :disabled="colorMode !== 'custom'"
          />
        </div>
      </div>
      <div class="profile-about">
        <label
          class="field-label"
          for="profile-bio"
          >About you</label
        ><small
          class="field-hint"
          aria-live="polite"
          >{{ bio.length }}/300</small
        >
      </div>
      <textarea
        id="profile-bio"
        v-model="bio"
        class="text-input"
        rows="3"
        maxlength="300"
        placeholder="A line or two people see on your profile"
      />
      <div class="form-actions">
        <p
          v-if="error"
          class="form-error"
          role="alert"
        >
          {{ error }}
        </p>
        <span
          v-else-if="saved"
          class="success-text"
          role="status"
          ><AppIcon
            name="check"
            :size="15"
          />Profile saved</span
        ><button
          class="button primary"
          :disabled="!dirty || chat.profile.isLoading || processingAvatar"
        >
          {{ chat.profile.isLoading ? 'Saving…' : 'Save changes' }}
        </button>
      </div>
    </form>
    <NetworkIdentity />
  </div>
</template>
