<script setup lang="ts">
import { memberNameColor } from '../../shared/utils/identity'
/** Settings → Profile: live identity on the current network, then the IRCord profile. */
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
  <div>
    <NetworkIdentity />
    <form
      class="profile-form"
      @submit.prevent="saveProfile"
    >
      <h4 class="settings-subtitle">IRCord profile</h4>
      <p class="field-hint">
        Shown to other IRCord users. IRC networks only see your nickname and real name.
      </p>
      <div class="profile-preview">
        <UserAvatar
          :member="previewMember"
          size="large"
        /><span
          ><strong :style="nameStyle(previewMember)">{{ name || 'Your name' }}</strong
          ><small>@{{ user?.nick }}</small></span
        >
      </div>
      <div class="profile-avatar-actions">
        <input
          ref="avatarInput"
          class="sr-only"
          type="file"
          accept="image/png,image/jpeg,image/webp"
          aria-label="Upload profile image"
          :disabled="processingAvatar || chat.profile.isLoading"
          @change="chooseAvatar"
        />
        <button
          type="button"
          class="button secondary small"
          :disabled="processingAvatar || chat.profile.isLoading"
          @click="avatarInput?.click()"
        >
          {{ processingAvatar ? 'Preparing image…' : 'Change image' }}
        </button>
        <button
          v-if="previewMember?.avatarUrl"
          type="button"
          class="text-button"
          :disabled="processingAvatar || chat.profile.isLoading"
          @click="removeAvatar"
        >
          Remove image
        </button>
      </div>
      <span class="field-hint">PNG, JPEG or WebP, up to 5 MB.</span>
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
      <fieldset class="profile-name-color">
        <legend>Name color</legend>
        <div class="name-color-options">
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
        <p class="field-hint">Adjusted for readability in every theme.</p>
      </fieldset>
      <label
        class="field-label"
        for="profile-bio"
        >About you</label
      ><textarea
        id="profile-bio"
        v-model="bio"
        class="text-input"
        rows="4"
        maxlength="300"
      /><small class="field-hint">{{ bio.length }}/300 characters</small>
      <div class="form-actions">
        <span
          v-if="saved"
          class="success-text"
          role="status"
          ><AppIcon
            name="check"
            :size="15"
          />Profile saved</span
        ><button
          class="button primary"
          :disabled="chat.profile.isLoading || processingAvatar"
        >
          {{ chat.profile.isLoading ? 'Saving…' : 'Save changes' }}
        </button>
      </div>
    </form>
    <p
      v-if="error"
      class="form-error"
      role="alert"
    >
      {{ error }}
    </p>
  </div>
</template>
