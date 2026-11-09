<script setup lang="ts">
/** Network dialog → Login: how IRCord identifies you, and its secret. */
const { form, editing, providedPassword, passwordSet, clearPassword, methods } = useNetworkForm()
</script>
<template>
  <fieldset>
    <legend>{{ $t('networkForm.login') }}</legend>
    <AppSegmented
      v-model="form.method"
      class="auth-methods flex-wrap mt-2.5"
      :label="$t('networkForm.loginMethod')"
      :options="methods"
    />
    <FieldHint>
      {{ methods.find((m) => m.value === form.method)?.note }}
    </FieldHint>
    <div
      v-if="form.method !== 'none' && form.method !== 'sasl-external'"
      class="field-grid grid [grid-template-columns:1fr_1fr] gap-x-3 [align-items:start] max-[700px]:[grid-template-columns:1fr]"
    >
      <AppInput
        v-if="form.method === 'sasl-plain'"
        id="network-account"
        v-model="form.account"
        :label="$t('networkForm.account')"
        :placeholder="form.nick"
        required
        autocomplete="username"
      />
      <AppField
        id="network-password"
        :label="$t('networkForm.password')"
        :class="{ 'col-span-2 max-[700px]:col-auto': form.method !== 'sasl-plain' }"
      >
        <template #default="control">
          <UiInput
            v-bind="control"
            v-model="form.password"
            type="password"
            :placeholder="
              providedPassword
                ? $t('networkForm.providedPassword')
                : passwordSet && !clearPassword
                  ? $t('networkForm.savedPassword')
                  : ''
            "
            :required="!passwordSet || clearPassword"
            autocomplete="new-password"
          />
          <UiButton
            v-if="editing && passwordSet && !form.password"
            variant="link"
            size="text"
            type="button"
            class="justify-self-start"
            @click="clearPassword = !clearPassword"
          >
            {{ clearPassword ? $t('networkForm.keepPassword') : $t('networkForm.forgetPassword') }}
          </UiButton>
        </template>
      </AppField>
    </div>
    <FieldHint v-if="form.method === 'sasl-external'">
      {{ $t('networkForm.certificateUnavailable') }}
    </FieldHint>
  </fieldset>
</template>
