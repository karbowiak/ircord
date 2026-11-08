<script setup lang="ts">
/** Network dialog → On connect: channels to join and commands to send. */
const { form } = useNetworkForm()
</script>
<template>
  <fieldset>
    <legend>On connect</legend>
    <div class="field-grid grid [grid-template-columns:1fr_1fr] gap-x-3 [align-items:start]">
      <div>
        <FieldLabel for="network-autojoin">Join these channels</FieldLabel
        ><UiTextarea
          id="network-autojoin"
          v-model="form.autoJoin"
          rows="4"
          placeholder="#channel&#10;#private secretkey"
          spellcheck="false"
        />
      </div>
      <div>
        <FieldLabel for="network-perform">Then send these commands</FieldLabel
        ><UiTextarea
          id="network-perform"
          v-model="form.perform"
          class="mono"
          rows="4"
          placeholder="MODE mynick +x"
          spellcheck="false"
        />
      </div>
    </div>
    <FieldHint> One per line. Commands are raw IRC lines, without a leading slash. </FieldHint>
  </fieldset>
</template>
