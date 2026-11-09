<script setup lang="ts">
import type { NetworkInput } from '../../shared/types/chat'
/**
 * Adding or editing a network, or the first connection's short setup. The form's state lives in
 * useNetworkForm; each fieldset is its own component sharing it.
 */
const props = withDefaults(
  defineProps<{ networkId?: string; prefill?: Partial<NetworkInput>; firstConnection?: boolean }>(),
  { networkId: '', prefill: () => ({}), firstConnection: false },
)
const emit = defineEmits<{ close: [] }>()
const {
  existing,
  editing,
  presets,
  advanced,
  revealInvalidField,
  error,
  saving,
  verification,
  checkAttempt,
  needsVerification,
  selectPreset,
  submit,
  remove,
} = createNetworkForm(props, () => emit('close'))
</script>
<template>
  <AppDialog
    :title="
      editing
        ? $t('networkForm.editTitle', { name: existing?.name ?? '' })
        : firstConnection
          ? $t('networkForm.firstTitle')
          : $t('networks.add')
    "
    :wide="!firstConnection || advanced"
    @close="emit('close')"
  >
    <form
      class="dialog-form network-form pt-0 pr-5 pb-5 pl-5 [&_.field-label]:mt-2.5 [&_fieldset]:mt-[18px] [&_fieldset]:border-0 [&_fieldset]:p-0 [&_legend]:mb-1 [&_legend]:w-full [&_legend]:[border-bottom:1px_solid_var(--border)] [&_legend]:pb-1.5 [&_legend]:text-[12px] [&_legend]:font-[650] [&_legend]:uppercase [&_legend]:tracking-[0.05em] [&_legend]:text-faint"
      @submit.prevent="submit"
      @invalid.capture="revealInvalidField"
    >
      <NetworkQuickSetup v-if="firstConnection" />
      <div
        v-show="!firstConnection || advanced"
        id="network-connection-settings"
        class="network-connection-settings flow-root"
      >
        <div
          v-if="!editing && presets.length"
          class="network-presets flex gap-1.5 flex-wrap"
        >
          <button
            v-for="preset in presets"
            :key="preset.name"
            type="button"
            class="flex items-center gap-2 rounded-full bg-secondary py-1 pr-2.5 pl-1 text-[13px] hover:bg-accent"
            @click="selectPreset(preset)"
          >
            <span
              aria-hidden="true"
              class="flex size-6 items-center justify-center rounded-full text-[12px] font-[650]"
              :class="`tone-${preset.color}`"
              >{{ preset.initial }}</span
            >{{ preset.name }}
          </button>
        </div>
        <NetworkServerFields />
        <NetworkIdentityFields />
        <NetworkLoginFields />
        <NetworkOnConnectFields />
      </div>

      <UiAlert
        v-if="error"
        as="p"
        variant="destructive"
        presentation="inline"
        class="form-error mt-3"
      >
        {{ error }}
      </UiAlert>
      <FieldHint
        v-if="needsVerification && (verification.detail || verification.state === 'checking')"
        :class="{ 'text-destructive': verification.state === 'error' }"
        role="status"
      >
        {{
          verification.state === 'checking'
            ? $t('networkForm.verifyingAddress')
            : verification.detail
        }}
        <UiButton
          v-if="verification.state === 'error'"
          variant="link"
          size="text"
          type="button"
          @click="checkAttempt++"
        >
          {{ $t('networkForm.retryVerification') }}
        </UiButton>
      </FieldHint>
      <div class="form-actions flex justify-end items-center gap-2 mt-5">
        <UiButton
          v-if="editing"
          variant="secondary-destructive"
          type="button"
          @click="remove"
        >
          {{ $t('networkForm.removeNetwork') }}
        </UiButton>
        <span class="flex-1" />
        <UiButton
          variant="secondary"
          type="button"
          @click="emit('close')"
        >
          {{ firstConnection ? $t('networkForm.setupLater') : $t('common.cancel') }}</UiButton
        ><UiButton :disabled="saving || (needsVerification && verification.state !== 'verified')">
          {{
            verification.state === 'checking'
              ? $t('networkForm.verifying')
              : saving
                ? $t('common.saving')
                : editing
                  ? $t('common.saveChanges')
                  : firstConnection
                    ? $t('networkForm.connect')
                    : $t('networkForm.addNetwork')
          }}
        </UiButton>
      </div>
    </form>
  </AppDialog>
</template>
