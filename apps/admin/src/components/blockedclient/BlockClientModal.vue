<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import {
  NAlert,
  NButton,
  NInput,
  NInputNumber,
  NModal,
  NRadioButton,
  NRadioGroup,
  NSelect,
  useMessage
} from "naive-ui";
import { createBlockedClient } from "@/api/blockedClient";
import { apiErrorMessage } from "@/api/http";
import { BLOCK_DURATION_PRESETS, BLOCK_DURATION_UNIT_OPTIONS } from "@/constants/enums";
import { formatUnixDateTime } from "@/lib/datetime";
import { ipOrCidrError } from "@/lib/ip";
import type { BlockedClientMain } from "@/api/types";

const props = withDefaults(
  defineProps<{
    show: boolean;
    // the active blocks, to warn that blocking one of them again prolongs it
    active?: BlockedClientMain[];
    // prefills the address, e.g. when blocking a client from the Security page
    ip?: string;
  }>(),
  { active: () => [], ip: "" }
);
const emit = defineEmits<{ "update:show": [value: boolean]; saved: [row: BlockedClientMain] }>();

const message = useMessage();

const CUSTOM = -1;

const form = reactive({
  ip: "",
  reason: "",
  // seconds of a preset, 0 = no end, CUSTOM = customAmount × customUnit
  preset: 86400,
  customAmount: 12 as number | null,
  customUnit: 3600
});
const touched = reactive({ ip: false, reason: false });
const submitting = ref(false);
const serverError = ref("");

watch(
  () => props.show,
  (show) => {
    if (!show) return;
    form.ip = props.ip;
    form.reason = "";
    form.preset = 86400;
    form.customAmount = 12;
    form.customUnit = 3600;
    touched.ip = false;
    touched.reason = false;
    serverError.value = "";
  }
);

const durationSeconds = computed(() => {
  if (form.preset !== CUSTOM) return form.preset;
  return Math.max(0, Math.trunc(form.customAmount || 0)) * form.customUnit;
});

const ipError = computed(() => ipOrCidrError(form.ip));
const reasonError = computed(() => (form.reason.trim() ? "" : "reason is required"));
const durationError = computed(() =>
  form.preset === CUSTOM && durationSeconds.value <= 0 ? "duration must be at least 1 minute" : ""
);
const valid = computed(() => !ipError.value && !reasonError.value && !durationError.value);

const untilText = computed(() => {
  if (durationSeconds.value <= 0) return form.preset === 0 ? "Stays until someone unblocks it." : "";
  const until = Math.floor(Date.now() / 1000) + durationSeconds.value;
  return `Lasts until ${formatUnixDateTime(until)}.`;
});

// The server keeps a single address as /32 (/128 for IPv6): match the entered
// value against the active list the same way, case-insensitively for IPv6.
function asNetwork(value: string): string {
  const ip = value.trim().toLowerCase();
  if (!ip || ip.includes("/")) return ip;
  return `${ip}/${ip.includes(":") ? 128 : 32}`;
}

const existing = computed(() => {
  if (ipError.value) return null;
  const network = asNetwork(form.ip);
  return props.active.find((row) => row.ip.toLowerCase() === network) ?? null;
});

function close(): void {
  emit("update:show", false);
}

async function submit(): Promise<void> {
  touched.ip = true;
  touched.reason = true;
  if (!valid.value) return;
  submitting.value = true;
  serverError.value = "";
  try {
    const prolonged = Boolean(existing.value);
    const row = await createBlockedClient({
      ip: form.ip.trim(),
      reason: form.reason.trim(),
      duration_seconds: durationSeconds.value
    });
    message.success(`${row.ip || form.ip.trim()} ${prolonged ? "block prolonged" : "blocked"}`);
    emit("saved", row);
    close();
  } catch (error) {
    serverError.value = apiErrorMessage(error, "Failed to block the address");
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="Block address"
    class="block-modal"
    :bordered="false"
    :mask-closable="!submitting"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <form class="block-form" @submit.prevent="submit">
      <label class="field">
        <span class="field__label">Address or network</span>
        <NInput
          v-model:value="form.ip"
          placeholder="203.0.113.7, 203.0.113.0/24 or 2001:db8:1:2::/64"
          :status="touched.ip && ipError ? 'error' : undefined"
          :disabled="submitting"
          :input-props="{ autocomplete: 'off', spellcheck: false }"
          @blur="touched.ip = true"
        />
        <span v-if="touched.ip && ipError" class="field-error">{{ ipError }}</span>
        <span v-else-if="existing" class="field__note">
          Already blocked
          {{ existing.expires_at_unix ? `until ${formatUnixDateTime(existing.expires_at_unix)}` : "with no end" }}
          — blocking it again prolongs the block.
        </span>
      </label>

      <label class="field">
        <span class="field__label">Reason</span>
        <NInput
          v-model:value="form.reason"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 5 }"
          placeholder="e.g. credential stuffing on /auth/login"
          :status="touched.reason && reasonError ? 'error' : undefined"
          :disabled="submitting"
          @blur="touched.reason = true"
        />
        <span v-if="touched.reason && reasonError" class="field-error">{{ reasonError }}</span>
      </label>

      <div class="field">
        <span class="field__label">Duration</span>
        <NRadioGroup v-model:value="form.preset" :disabled="submitting" class="block-form__presets">
          <NRadioButton
            v-for="preset in BLOCK_DURATION_PRESETS"
            :key="preset.seconds"
            :value="preset.seconds"
            :label="preset.label"
          />
          <NRadioButton :value="CUSTOM" label="custom" />
        </NRadioGroup>
        <div v-if="form.preset === CUSTOM" class="block-form__custom">
          <NInputNumber
            v-model:value="form.customAmount"
            :min="1"
            :step="1"
            :precision="0"
            :disabled="submitting"
            :status="durationError ? 'error' : undefined"
            aria-label="Duration amount"
          />
          <NSelect
            v-model:value="form.customUnit"
            :options="BLOCK_DURATION_UNIT_OPTIONS"
            :disabled="submitting"
            aria-label="Duration unit"
          />
        </div>
        <span v-if="durationError" class="field-error">{{ durationError }}</span>
        <span v-else class="field__note">{{ untilText }}</span>
      </div>

      <NAlert v-if="serverError" type="error" :bordered="false">{{ serverError }}</NAlert>

      <p class="block-form__hint muted">
        The block takes effect on all gateways within seconds, without a Deploy: requests from the
        address get <code>403</code> on every route. Trusted clients from the Root configuration
        can't be blocked, and too wide networks are refused.
      </p>

      <!-- lets Enter in the address field submit the form -->
      <button type="submit" hidden />
    </form>

    <template #footer>
      <div class="form-actions">
        <NButton :disabled="submitting" @click="close">Cancel</NButton>
        <NButton type="error" :loading="submitting" @click="submit">
          {{ existing ? "Prolong block" : "Block" }}
        </NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
:global(.block-modal) {
  width: min(520px, calc(100vw - 32px));
}

.block-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.field__label {
  font-size: 12px;
  color: var(--c-text-3);
}

.field__note {
  font-size: 12px;
  color: var(--c-text-2);
}

.block-form__presets {
  display: flex;
  flex-wrap: wrap;
}

.block-form__custom {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 8px;
  max-width: 320px;
}

.block-form__hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.45;
}

.block-form__hint code {
  font-family: var(--font-mono);
  color: var(--c-text-2);
}
</style>
