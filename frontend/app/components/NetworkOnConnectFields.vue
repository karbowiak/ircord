<script setup lang="ts">
/** Network dialog → On connect: channels to join and commands to send. */
const { form } = useNetworkForm()
</script>
<template>
  <fieldset>
    <legend>On connect</legend>
    <div class="field-grid grid [grid-template-columns:1fr_1fr] gap-x-3 [align-items:start]">
      <AppInput
        id="network-autojoin"
        v-model="form.autoJoin"
        label="Join these channels"
        multiline
        rows="4"
        placeholder="#channel&#10;#private secretkey"
        spellcheck="false"
      />
      <AppInput
        id="network-perform"
        v-model="form.perform"
        label="Then send these commands"
        multiline
        class="mono"
        rows="4"
        placeholder="MODE mynick +x"
        spellcheck="false"
      />
    </div>
    <FieldHint> One per line. Commands are raw IRC lines, without a leading slash. </FieldHint>
  </fieldset>
</template>
