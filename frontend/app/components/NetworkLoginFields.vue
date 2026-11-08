<script setup lang="ts">
/** Network dialog → Login: how IRCord identifies you, and its secret. */
const { form, editing, providedPassword, passwordSet, clearPassword, methods } = useNetworkForm()
</script>
<template>
  <fieldset>
    <legend>Login</legend>
    <AppSegmented
      v-model="form.method"
      class="auth-methods flex-wrap mt-2.5"
      label="Login method"
      :options="methods"
    />
    <FieldHint>
      {{ methods.find((m) => m.value === form.method)?.note }}
    </FieldHint>
    <div
      v-if="form.method !== 'none' && form.method !== 'sasl-external'"
      class="field-grid grid [grid-template-columns:1fr_1fr] gap-x-3 [align-items:start]"
    >
      <AppInput
        v-if="form.method === 'sasl-plain'"
        id="network-account"
        v-model="form.account"
        label="Account"
        :placeholder="form.nick"
        required
        autocomplete="username"
      />
      <AppField
        id="network-password"
        label="Password"
        :class="{ 'span-2': form.method !== 'sasl-plain' }"
      >
        <template #default="control">
          <UiInput
            v-bind="control"
            v-model="form.password"
            type="password"
            :placeholder="
              providedPassword
                ? 'Provided by server · type to override'
                : passwordSet && !clearPassword
                  ? 'Saved · type to replace'
                  : ''
            "
            :required="!passwordSet || clearPassword"
            autocomplete="new-password"
          />
          <UiButton
            variant="link"
            size="text"
            v-if="editing && passwordSet && !form.password"
            type="button"
            class="justify-self-start"
            @click="clearPassword = !clearPassword"
          >
            {{ clearPassword ? 'Keep saved password' : 'Forget saved password' }}
          </UiButton>
        </template>
      </AppField>
    </div>
    <FieldHint v-if="form.method === 'sasl-external'">
      Client certificates are configured on the server side and aren’t available yet.
    </FieldHint>
  </fieldset>
</template>
