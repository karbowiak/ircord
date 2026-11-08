<script setup lang="ts">
import type { Member } from '../../shared/types/chat'
import type { SignupAvatar } from '../stores/auth'
import {
  DEFAULT_AVATAR_PATH,
  defaultAvatarUrl,
  suggestAvatar,
  type DefaultAvatar,
} from '../../shared/utils/default-avatars'
/**
 * Sign in or create an account with a username and password. The username is also your nick
 * and account on irc.ircord.com, so it's checked against the network while you type.
 */
const auth = useAuthStore()
const mode = ref<'signin' | 'signup'>('signin')
const username = ref('')
const password = ref('')
const displayName = ref('')
const registrationKey = ref('')
const error = ref('')
const first = ref<HTMLInputElement | null>(null)

const registration = computed(() => auth.registration.data)
const busy = computed(() => auth.login.isLoading || auth.signup.isLoading)

function go(next: typeof mode.value) {
  mode.value = next
  error.value = ''
  password.value = ''
  registrationKey.value = ''
  nextTick(() => first.value?.focus())
}

const { check, checking } = useUsernameCheck(username, () => mode.value === 'signup')

const canSignUp = computed(
  () =>
    !!username.value.trim() &&
    !checking.value &&
    check.value?.available !== false &&
    password.value.length >= 8 &&
    (!registration.value?.registrationKeyRequired || !!registrationKey.value.trim()),
)

async function attempt(action: () => Promise<unknown>) {
  error.value = ''
  try {
    await action()
  } catch (e) {
    error.value = errorText(e)
  }
}
const signIn = () =>
  attempt(() =>
    auth.login.mutateAsync({ username: username.value.trim(), password: password.value }),
  )
// Profile picture: a default suggested from the username (it follows the name as it's typed)
// until one is picked, an uploaded image, or none.
const { avatars } = useDefaultAvatars()
const suggestion = computed(() => suggestAvatar(username.value, avatars.value))
type Picture =
  | { kind: 'default'; avatar: DefaultAvatar }
  | { kind: 'upload'; dataUrl: string }
  | { kind: 'none' }
const picked = ref<Picture | null>(null)
const picture = computed<Picture | null>(
  () => picked.value ?? (suggestion.value ? { kind: 'default', avatar: suggestion.value } : null),
)
const pictureUrl = computed(() =>
  picture.value?.kind === 'default'
    ? DEFAULT_AVATAR_PATH + picture.value.avatar.file
    : picture.value?.kind === 'upload'
      ? picture.value.dataUrl
      : undefined,
)
const previewMember = computed(
  () =>
    ({
      displayName: displayName.value.trim() || username.value.trim() || '?',
      avatarUrl: pictureUrl.value,
    }) as Member,
)
const pictureInput = ref<HTMLInputElement | null>(null)
const preparingPicture = ref(false)
async function uploadPicture(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  error.value = ''
  preparingPicture.value = true
  try {
    picked.value = { kind: 'upload', dataUrl: await prepareAvatarImage(file) }
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'This image could not be opened.'
  } finally {
    preparingPicture.value = false
    if (pictureInput.value) pictureInput.value.value = ''
  }
}
function signupAvatar(): SignupAvatar | undefined {
  const value = picture.value
  if (value?.kind === 'default')
    return { avatarUrl: defaultAvatarUrl(value.avatar, location.origin) }
  if (value?.kind === 'upload') return { avatarDataUrl: value.dataUrl }
  return undefined
}

const signUp = () =>
  attempt(() =>
    auth.signup.mutateAsync({
      username: username.value.trim(),
      password: password.value,
      ...(displayName.value.trim() ? { displayName: displayName.value.trim() } : {}),
      ...(registrationKey.value.trim() ? { registrationKey: registrationKey.value.trim() } : {}),
      avatar: signupAvatar(),
    }),
  )
</script>
<template>
  <main
    class="login-screen h-full grid [place-items:safe_center] py-6 px-4 [background:radial-gradient(_60%_50%_at_50%_0%,_color-mix(in_srgb,_var(--accent)_14%,_transparent),_transparent_),_var(--rail)] overflow-y-auto overscroll-contain"
  >
    <div
      class="login-card w-[min(420px,_100%)] grid gap-3.5 [justify-items:center] pt-8 pr-7 pb-7 pl-7 rounded-lg bg-background shadow-popover text-center"
    >
      <BrandMark class="brand-mark block w-12 h-12 flex-none" />

      <template v-if="mode === 'signup'">
        <h1 class="text-[22px] font-[700] tracking-[-0.02em]">Create your account</h1>
        <p class="login-copy text-[14px] text-muted-foreground leading-[1.55]">
          One account for IRCord and irc.ircord.com. You’re logged in there from the start, and the
          same username and password work in any IRC client.
        </p>
        <form
          class="login-form grid gap-2.5 w-full text-left"
          @submit.prevent="signUp"
        >
          <AppField
            id="signup-username"
            label="Username"
            help-live
            :help-class="{ 'text-destructive': check?.available === false }"
          >
            <template #default="control">
              <UiInput
                v-bind="control"
                :ref="(el) => (first = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
                v-model="username"
                autocomplete="username"
                autocapitalize="none"
                spellcheck="false"
                maxlength="30"
                required
                :aria-invalid="check?.available === false"
              />
            </template>
            <template #help>
              <template v-if="check?.available === false">{{ check.reason }}</template>
              <template v-else-if="check?.available"
                ><AppIcon
                  name="check"
                  :size="13"
                  class="inline align-[-2px] text-success"
                />
                {{ username.trim() }} is available</template
              >
              <template v-else-if="checking">Checking…</template>
              <template v-else>Also your nick. You can set a display name later.</template>
            </template>
          </AppField>
          <AppInput
            id="signup-display-name"
            v-model="displayName"
            help="How you appear in chat. Spaces are fine; you can change it later."
            maxlength="80"
            autocomplete="nickname"
          >
            <template #label>Display name <small>(optional)</small></template>
          </AppInput>
          <fieldset class="signup-picture grid gap-2 min-w-0 mb-1 p-0 border-0">
            <FieldLabel
              as="legend"
              class="m-0"
              >Profile picture</FieldLabel
            >
            <div
              class="signup-picture-current grid grid-cols-[auto_1fr] items-center gap-x-3.5 gap-y-1"
            >
              <UserAvatar
                :member="previewMember"
                size="large"
                class="row-span-2"
              />
              <p
                class="field-hint m-0 text-[12px] text-faint"
                aria-live="polite"
              >
                <template v-if="picture?.kind === 'upload'">Your own image.</template>
                <template v-else-if="picture?.kind === 'none'">No picture for now.</template>
                <template v-else-if="picked">You picked this one.</template>
                <template v-else-if="suggestion"
                  >Suggested for {{ username.trim() }}. Pick another if you like.</template
                >
                <template v-else-if="avatars.length"
                  >Type a username for a suggestion, or pick one.</template
                >
                <template v-else>Upload one if you like, or add it later.</template>
              </p>
              <div class="signup-picture-actions flex flex-wrap items-center gap-2.5">
                <UiButton
                  type="button"
                  variant="secondary"
                  size="sm"
                  :disabled="preparingPicture || busy"
                  @click="pictureInput?.click()"
                >
                  <AppIcon
                    name="upload"
                    :size="14"
                  />{{ preparingPicture ? 'Preparing…' : 'Upload your own' }}
                </UiButton>
                <UiButton
                  v-if="picture && picture.kind !== 'none'"
                  type="button"
                  variant="link"
                  size="text"
                  :disabled="busy"
                  @click="picked = { kind: 'none' }"
                >
                  No picture
                </UiButton>
              </div>
              <input
                ref="pictureInput"
                class="sr-only"
                type="file"
                accept="image/png,image/jpeg,image/webp"
                aria-label="Upload your own profile picture"
                tabindex="-1"
                @change="uploadPicture"
              />
            </div>
            <DefaultAvatarPicker
              v-if="avatars.length"
              :avatars="avatars"
              :selected="picture?.kind === 'default' ? picture.avatar.id : undefined"
              :suggested="picked ? undefined : suggestion?.id"
              name="signup-avatar"
              label="Default profile pictures"
              :disabled="busy"
              @select="picked = { kind: 'default', avatar: $event }"
            />
          </fieldset>
          <AppInput
            id="signup-password"
            v-model="password"
            label="Password"
            help="At least 8 characters, no spaces."
            type="password"
            autocomplete="new-password"
            minlength="8"
            maxlength="128"
            required
          />
          <AppInput
            v-if="registration?.registrationKeyRequired"
            id="registration-key"
            v-model="registrationKey"
            label="Registration key"
            help="Registration is limited for now. Enter the key shared with you."
            type="text"
            autocomplete="off"
            autocapitalize="none"
            spellcheck="false"
            maxlength="256"
            required
          />
          <UiButton
            class="w-full"
            :disabled="!canSignUp || busy"
          >
            {{ auth.signup.isLoading ? 'Creating your account…' : 'Create account' }}
          </UiButton>
        </form>
        <p class="login-switch text-[13px] text-muted-foreground">
          Already have an account?
          <UiButton
            variant="link"
            size="text"
            @click="go('signin')"
          >
            Sign in
          </UiButton>
        </p>
      </template>

      <template v-else>
        <h1 class="text-[22px] font-[700] tracking-[-0.02em]">Sign in to IRCord</h1>
        <p class="login-copy text-[14px] text-muted-foreground leading-[1.55]">
          Your IRC networks in one place, connected even when you’re not.
        </p>
        <form
          class="login-form grid gap-2.5 w-full text-left"
          @submit.prevent="signIn"
        >
          <AppField
            id="signin-username"
            label="Username"
          >
            <template #default="control">
              <UiInput
                v-bind="control"
                :ref="(el) => (first = (el as { $el: HTMLInputElement } | null)?.$el ?? null)"
                v-model="username"
                autocomplete="username"
                autocapitalize="none"
                spellcheck="false"
                required
              />
            </template>
          </AppField>
          <AppInput
            id="signin-password"
            v-model="password"
            label="Password"
            type="password"
            autocomplete="current-password"
            required
          />
          <UiButton
            class="w-full"
            :disabled="!username.trim() || !password || busy"
          >
            {{ auth.login.isLoading ? 'Signing in…' : 'Sign in' }}
          </UiButton>
        </form>
        <p
          v-if="registration?.registrationEnabled"
          class="login-switch text-[13px] text-muted-foreground"
        >
          New here?
          <UiButton
            variant="link"
            size="text"
            @click="go('signup')"
          >
            Create an account
          </UiButton>
        </p>
        <p
          v-else-if="auth.registration.error"
          class="form-error text-destructive text-[13px] mt-3"
        >
          Registration availability could not be checked.
          <UiButton
            variant="link"
            size="text"
            @click="auth.registration.refetch()"
          >
            Try again
          </UiButton>
        </p>
        <p
          v-else-if="registration"
          class="login-note text-[12px] text-faint"
        >
          New registrations are currently closed.
        </p>
      </template>

      <UiAlert
        as="p"
        variant="destructive"
        presentation="inline"
        v-if="error"
        class="form-error mt-3"
      >
        {{ error }}
      </UiAlert>
    </div>
  </main>
</template>
