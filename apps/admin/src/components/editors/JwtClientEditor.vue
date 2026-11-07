<script setup lang="ts">
import { ref, watch } from "vue";
import { NInput } from "naive-ui";
import type { JwtClient } from "@/api/types";

const props = defineProps<{ modelValue: JwtClient }>();
const emit = defineEmits<{ (e: "update:modelValue", value: JwtClient): void }>();

function set(patch: Partial<JwtClient>): void {
  emit("update:modelValue", { ...props.modelValue, ...patch });
}

function parseScopes(text: string): string[] {
  return text.split(/[\s,]+/).filter(Boolean);
}

// Scopes are edited as free text so a trailing space survives typing; the
// array in the model follows the text.
const scopesText = ref(props.modelValue.scopes.join(" "));
watch(
  () => props.modelValue.scopes.join(" "),
  (value) => {
    if (value !== parseScopes(scopesText.value).join(" ")) {
      scopesText.value = value;
    }
  }
);

function setScopes(text: string): void {
  scopesText.value = text;
  set({ scopes: parseScopes(text) });
}
</script>

<template>
  <div class="client-grid">
    <label class="field">
      <span class="field__label">Client id</span>
      <NInput :value="modelValue.id" placeholder="ruto" @update:value="(value: string) => set({ id: value })" />
    </label>
    <label class="field">
      <span class="field__label">Client secret (a variable reference works too)</span>
      <NInput
        :value="modelValue.secret"
        type="password"
        show-password-on="click"
        placeholder="client secret"
        @update:value="(value: string) => set({ secret: value })"
      />
    </label>
    <label class="field client-grid__scopes">
      <span class="field__label">Scopes (space separated; openid is always asked for)</span>
      <NInput :value="scopesText" placeholder="openid profile email" @update:value="setScopes" />
    </label>
  </div>
</template>

<style scoped>
.client-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  width: 100%;
}

.client-grid__scopes {
  grid-column: 1 / -1;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field__label {
  font-size: 12px;
  color: var(--c-text-3);
}

@media (max-width: 560px) {
  .client-grid {
    grid-template-columns: 1fr;
  }
}
</style>
