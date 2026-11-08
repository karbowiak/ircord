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
        ? `Edit ${existing?.name}`
        : firstConnection
          ? 'Your first connection'
          : 'Add a network'
    "
    :wide="!firstConnection || advanced"
    @close="emit('close')"
  >
    <form
      class="dialog-form pt-0 pr-5 pb-5 pl-5 network-form"
      @submit.prevent="submit"
      @invalid.capture="revealInvalidField"
    >
      <NetworkQuickSetup v-if="firstConnection" />
      <div
        id="network-connection-settings"
        v-show="!firstConnection || advanced"
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
        as="p"
        variant="destructive"
        presentation="inline"
        v-if="error"
        class="form-error mt-3"
      >
        {{ error }}
      </UiAlert>
      <FieldHint
        v-if="needsVerification && verification.detail"
        :class="{ 'danger-text': verification.state === 'error' }"
        role="status"
      >
        {{ verification.detail }}
        <UiButton
          variant="link"
          size="text"
          v-if="verification.state === 'error'"
          type="button"
          @click="checkAttempt++"
        >
          Retry verification
        </UiButton>
      </FieldHint>
      <div class="form-actions flex justify-end items-center gap-2 mt-5">
        <UiButton
          variant="secondary-destructive"
          v-if="editing"
          type="button"
          @click="remove"
        >
          Remove network
        </UiButton>
        <span class="spacer" />
        <UiButton
          variant="secondary"
          type="button"
          @click="emit('close')"
        >
          {{ firstConnection ? 'Set up later' : 'Cancel' }}</UiButton
        ><UiButton :disabled="saving || (needsVerification && verification.state !== 'verified')">
          {{
            verification.state === 'checking'
              ? 'Verifying…'
              : saving
                ? 'Saving…'
                : editing
                  ? 'Save changes'
                  : firstConnection
                    ? 'Connect'
                    : 'Add network'
          }}
        </UiButton>
      </div>
    </form>
  </AppDialog>
</template>
