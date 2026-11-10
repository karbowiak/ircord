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
const { t, locale } = useI18n()
const mode = ref<'signin' | 'signup'>('signin')
const username = ref('')
const password = ref('')
const displayName = ref('')
const registrationKey = ref('')
const error = ref('')
const first = ref<HTMLInputElement | null>(null)

const registration = computed(() => auth.registration.data)
const busy = computed(() => auth.login.isLoading || auth.signup.isLoading)

/**
 * Why a field stopped the form, by field id. Submit stays enabled: the browser's own required and
 * minlength checks run first and focus the first invalid field, whose message is kept here, under
 * the field, until it is edited. Only the first invalid field of an attempt gets a message, so one
 * error is announced at a time.
 */
const invalid = reactive<Record<string, string>>({})
const fieldError = (id: string) => {
  const value = invalid[id]
  return value?.startsWith('i18n:') ? t(value.slice(5)) : value
}
let reported = false
function onInvalid(event: Event) {
  const field = event.target as HTMLInputElement
  if (reported || !field.id) return
  reported = true
  // Each invalid event gets its own microtask checkpoint, so wait for the next task.
  setTimeout(() => (reported = false))
  invalid[field.id] = field.validationMessage
}
function onEdit(event: Event) {
  delete invalid[(event.target as HTMLElement).id]
}
/** Fields that are only spaces pass the browser's required check; stop at the first one. */
function blank(fields: [id: string, value: string, message: string][]) {
  const empty = fields.find(([, value]) => !value.trim())
  if (!empty) return false
  invalid[empty[0]] = empty[2]
  document.getElementById(empty[0])?.focus()
  return true
}

function go(next: typeof mode.value) {
  mode.value = next
  error.value = ''
  for (const id in invalid) delete invalid[id]
  password.value = ''
  registrationKey.value = ''
  nextTick(() => first.value?.focus())
}

const { check, checking } = useUsernameCheck(username, () => mode.value === 'signup')
// The help line shows “Checking…” while typing; screen readers hear only the answer.
useAnnouncedStatus(
  () =>
    mode.value !== 'signup' || !check.value
      ? ''
      : check.value.available
        ? t('login.available', { username: username.value.trim() })
        : (check.value.reason ?? ''),
  { delay: 300 },
)

async function attempt(action: () => Promise<unknown>) {
  error.value = ''
  try {
    await action()
  } catch (e) {
    error.value = errorText(e, (key) => t(key))
  }
}
function signIn() {
  if (blank([['signin-username', username.value, 'i18n:login.enterYourUsername']])) return
  return attempt(() =>
    auth.login.mutateAsync({ username: username.value.trim(), password: password.value }),
  )
}
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
    picked.value = {
      kind: 'upload',
      dataUrl: await prepareAvatarImage(file, (key, values) => t(key, values ?? {}), locale.value),
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('login.imageOpenFailed')
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

function signUp() {
  const fields: [string, string, string][] = [
    ['signup-username', username.value, 'i18n:login.enterUsername'],
  ]
  if (registration.value?.registrationKeyRequired)
    fields.push(['registration-key', registrationKey.value, 'i18n:login.enterRegistrationKey'])
  if (blank(fields)) return
  // A name the network already refused is not sent; the field says why and takes focus.
  if (check.value?.available === false) {
    invalid['signup-username'] = check.value.reason ?? 'i18n:login.usernameUnavailable'
    return first.value?.focus()
  }
  return createAccount()
}
const createAccount = () =>
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
        <h1 class="text-[22px] font-[700] tracking-[-0.02em]">{{ $t('login.createTitle') }}</h1>
        <p class="login-copy text-[14px] text-muted-foreground leading-[1.55]">
          {{ $t('login.createDescription') }}
        </p>
        <form
          class="login-form grid gap-2.5 w-full text-left"
          @submit.prevent="signUp"
          @invalid.capture="onInvalid"
          @input.capture="onEdit"
        >
          <AppField
            id="signup-username"
            :label="$t('login.username')"
            :error="fieldError('signup-username')"
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
                :aria-invalid="
                  check?.available === false || !!invalid['signup-username'] || undefined
                "
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
                {{ $t('login.available', { username: username.trim() }) }}</template
              >
              <template v-else-if="checking">{{ $t('login.checking') }}</template>
              <template v-else>{{ $t('login.usernameHelp') }}</template>
            </template>
          </AppField>
          <AppInput
            id="signup-display-name"
            v-model="displayName"
            :help="$t('login.displayNameHelp')"
            maxlength="80"
            autocomplete="nickname"
          >
            <template #label
              >{{ $t('login.displayName') }} <small>{{ $t('login.optional') }}</small></template
            >
          </AppInput>
          <fieldset class="signup-picture grid gap-2 min-w-0 mb-1 p-0 border-0">
            <FieldLabel
              as="legend"
              class="m-0"
              >{{ $t('login.profilePicture') }}</FieldLabel
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
                <template v-if="picture?.kind === 'upload'">{{ $t('login.ownImage') }}</template>
                <template v-else-if="picture?.kind === 'none'">{{
                  $t('login.noPictureNow')
                }}</template>
                <template v-else-if="picked">{{ $t('login.pickedImage') }}</template>
                <template v-else-if="suggestion">{{
                  $t('login.suggestedImage', { username: username.trim() })
                }}</template>
                <template v-else-if="avatars.length">{{ $t('login.suggestImageHelp') }}</template>
                <template v-else>{{ $t('login.uploadImageHelp') }}</template>
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
                  />{{ preparingPicture ? $t('login.preparing') : $t('login.uploadOwn') }}
                </UiButton>
                <UiButton
                  v-if="picture && picture.kind !== 'none'"
                  type="button"
                  variant="link"
                  size="text"
                  :disabled="busy"
                  @click="picked = { kind: 'none' }"
                >
                  {{ $t('login.noPicture') }}
                </UiButton>
              </div>
              <input
                ref="pictureInput"
                class="sr-only"
                type="file"
                accept="image/png,image/jpeg,image/webp"
                :aria-label="$t('login.uploadProfilePicture')"
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
              :label="$t('login.defaultPictures')"
              :disabled="busy"
              @select="picked = { kind: 'default', avatar: $event }"
            />
          </fieldset>
          <AppInput
            id="signup-password"
            v-model="password"
            :label="$t('login.password')"
            :help="$t('login.passwordHelp')"
            :error="fieldError('signup-password')"
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
            :label="$t('login.registrationKey')"
            :help="$t('login.registrationKeyHelp')"
            :error="fieldError('registration-key')"
            type="text"
            autocomplete="off"
            autocapitalize="none"
            spellcheck="false"
            maxlength="256"
            required
          />
          <UiButton
            class="w-full"
            :disabled="busy"
          >
            {{ auth.signup.isLoading ? $t('login.creating') : $t('login.createAccount') }}
          </UiButton>
        </form>
        <i18n-t
          keypath="mediaUi.login.alreadyHaveAccount"
          tag="p"
          class="login-switch text-[13px] text-muted-foreground"
        >
          <template #action>
            <UiButton
              variant="link"
              size="text"
              @click="go('signin')"
            >
              {{ $t('login.signIn') }}
            </UiButton>
          </template>
        </i18n-t>
      </template>

      <template v-else>
        <h1 class="text-[22px] font-[700] tracking-[-0.02em]">{{ $t('login.signInTitle') }}</h1>
        <p class="login-copy text-[14px] text-muted-foreground leading-[1.55]">
          {{ $t('login.signInDescription') }}
        </p>
        <form
          class="login-form grid gap-2.5 w-full text-left"
          @submit.prevent="signIn"
          @invalid.capture="onInvalid"
          @input.capture="onEdit"
        >
          <AppField
            id="signin-username"
            :label="$t('login.username')"
            :error="fieldError('signin-username')"
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
            :label="$t('login.password')"
            :error="fieldError('signin-password')"
            type="password"
            autocomplete="current-password"
            required
          />
          <UiButton
            class="w-full"
            :disabled="busy"
          >
            {{ auth.login.isLoading ? $t('login.signingIn') : $t('login.signIn') }}
          </UiButton>
        </form>
        <i18n-t
          v-if="registration?.registrationEnabled"
          keypath="mediaUi.login.newHere"
          tag="p"
          class="login-switch text-[13px] text-muted-foreground"
        >
          <template #action>
            <UiButton
              variant="link"
              size="text"
              @click="go('signup')"
            >
              {{ $t('login.createAnAccount') }}
            </UiButton>
          </template>
        </i18n-t>
        <i18n-t
          v-else-if="auth.registration.error"
          keypath="mediaUi.login.registrationFailedWithAction"
          tag="p"
          class="form-error text-destructive text-[13px] mt-3"
        >
          <template #action>
            <UiButton
              variant="link"
              size="text"
              @click="auth.registration.refetch()"
            >
              {{ $t('common.tryAgain') }}
            </UiButton>
          </template>
        </i18n-t>
        <p
          v-else-if="registration"
          class="login-note text-[12px] text-faint"
        >
          {{ $t('login.registrationClosed') }}
        </p>
      </template>

      <UiAlert
        v-if="error"
        as="p"
        variant="destructive"
        presentation="inline"
        class="form-error mt-3"
      >
        {{ error }}
      </UiAlert>
    </div>
  </main>
</template>
