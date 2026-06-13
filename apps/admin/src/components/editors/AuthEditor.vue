<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { NButton, NCard, NCheckbox, NIcon, NInput, NSelect, NSwitch } from "naive-ui";
import { RefreshOutline, TrashOutline } from "@vicons/ionicons5";
import { cloneAuthMethod, normalizeAuth } from "@/api/normalize";
import { createEmptyAuthMethod, arrayToLines, linesToArray } from "@/lib/forms";
import { generateSecret } from "@/lib/format";
import { AUTH_METHOD_TYPES, AUTH_MODE_OPTIONS } from "@/constants/enums";
import VariableInput from "./VariableInput.vue";
import type { Auth, AuthMethod, AuthMethodType, AuthMode, Variable } from "@/api/types";

const props = defineProps<{
  modelValue: Auth;
  jwtKidOptions?: string[];
  variableOptions?: Variable[];
  hideMode?: boolean;
}>();

const emit = defineEmits<{ "update:modelValue": [value: Auth] }>();

const localAuth = ref<Auth>(normalizeAuth(props.modelValue));
const jwtRolesText = ref<Record<number, string>>({});
const methodKeys = ref<string[]>([]);
const pushingToParent = ref(false);
let nextMethodKey = 0;

const jwtKidOptions = computed(() => {
  const source = props.jwtKidOptions || [];
  return Array.from(new Set(source.map((value) => value.trim()).filter(Boolean))).sort();
});

watch(
  () => props.modelValue,
  (value) => {
    if (pushingToParent.value) {
      pushingToParent.value = false;
      return;
    }
    localAuth.value = normalizeAuth(value);
    syncRolesText();
    syncMethodKeys();
  },
  { deep: true, immediate: true }
);

watch(
  localAuth,
  (value) => {
    pushingToParent.value = true;
    emit("update:modelValue", normalizeAuth(value));
  },
  { deep: true }
);

function syncRolesText(): void {
  const next: Record<number, string> = {};
  localAuth.value.methods.forEach((method, idx) => {
    next[idx] = arrayToLines(method.jwt?.roles);
  });
  jwtRolesText.value = next;
}

function syncMethodKeys(): void {
  const count = localAuth.value.methods.length;
  if (methodKeys.value.length < count) {
    methodKeys.value = [
      ...methodKeys.value,
      ...Array.from({ length: count - methodKeys.value.length }, () => {
        nextMethodKey += 1;
        return `auth-method-${nextMethodKey}`;
      })
    ];
  } else if (methodKeys.value.length > count) {
    methodKeys.value = methodKeys.value.slice(0, count);
  }
}

function setEnabled(value: boolean): void {
  localAuth.value = { ...localAuth.value, enabled: value };
}

function setMode(value: AuthMode): void {
  localAuth.value = { ...localAuth.value, mode: value };
}

function addMethod(type: AuthMethodType): void {
  localAuth.value = {
    ...localAuth.value,
    methods: [...localAuth.value.methods, createEmptyAuthMethod(type)]
  };
  syncRolesText();
  syncMethodKeys();
}

function removeMethod(methodIndex: number): void {
  localAuth.value = {
    ...localAuth.value,
    methods: localAuth.value.methods.filter((_, idx) => idx !== methodIndex)
  };
  methodKeys.value = methodKeys.value.filter((_, idx) => idx !== methodIndex);
  syncRolesText();
  syncMethodKeys();
}

function updateMethod(methodIndex: number, patch: Partial<AuthMethod>): void {
  localAuth.value = {
    ...localAuth.value,
    methods: localAuth.value.methods.map((method, idx) =>
      idx === methodIndex ? { ...cloneAuthMethod(method), ...patch } : method
    )
  };
}

function toggleType(methodIndex: number, type: AuthMethodType, enabled: boolean): void {
  const method = localAuth.value.methods[methodIndex];
  if (!method) return;
  if (type === "basic") {
    updateMethod(methodIndex, {
      basic: enabled ? method.basic || { users: [{ username: "", password: "" }] } : undefined
    });
  } else if (type === "api_key") {
    updateMethod(methodIndex, {
      api_key: enabled ? method.api_key || { header: "", keys: [{ name: "", key: "" }] } : undefined
    });
  } else if (type === "jwt") {
    updateMethod(methodIndex, { jwt: enabled ? method.jwt || { kid: "", roles: [] } : undefined });
  } else {
    updateMethod(methodIndex, {
      ip_validation: enabled
        ? method.ip_validation || { allowed_ips: [{ name: "", ip: "" }] }
        : undefined
    });
  }
}

// ---- Basic ----------------------------------------------------------------

function addBasicUser(methodIndex: number): void {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.basic) return;
  updateMethod(methodIndex, {
    basic: { ...method.basic, users: [...method.basic.users, { username: "", password: "" }] }
  });
}

function removeBasicUser(methodIndex: number, userIndex: number): void {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.basic) return;
  updateMethod(methodIndex, {
    basic: { ...method.basic, users: method.basic.users.filter((_, idx) => idx !== userIndex) }
  });
}

function setBasicField(
  methodIndex: number,
  userIndex: number,
  field: "username" | "password",
  value: string
): void {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.basic) return;
  const users = method.basic.users.map((user, idx) =>
    idx === userIndex ? { ...user, [field]: value } : user
  );
  updateMethod(methodIndex, { basic: { ...method.basic, users } });
}

// ---- API key --------------------------------------------------------------

function setApiHeader(methodIndex: number, value: string): void {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.api_key) return;
  updateMethod(methodIndex, { api_key: { ...method.api_key, header: value } });
}

function addApiKey(methodIndex: number): void {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.api_key) return;
  updateMethod(methodIndex, {
    api_key: { ...method.api_key, keys: [...method.api_key.keys, { name: "", key: "" }] }
  });
}

function removeApiKey(methodIndex: number, keyIndex: number): void {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.api_key) return;
  updateMethod(methodIndex, {
    api_key: { ...method.api_key, keys: method.api_key.keys.filter((_, idx) => idx !== keyIndex) }
  });
}

function setApiKeyField(
  methodIndex: number,
  keyIndex: number,
  field: "name" | "key",
  value: string
): void {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.api_key) return;
  const keys = method.api_key.keys.map((item, idx) =>
    idx === keyIndex ? { ...item, [field]: value } : item
  );
  updateMethod(methodIndex, { api_key: { ...method.api_key, keys } });
}

// ---- IP validation --------------------------------------------------------

function addAllowedIp(methodIndex: number): void {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.ip_validation) return;
  updateMethod(methodIndex, {
    ip_validation: {
      ...method.ip_validation,
      allowed_ips: [...method.ip_validation.allowed_ips, { name: "", ip: "" }]
    }
  });
}

function removeAllowedIp(methodIndex: number, ipIndex: number): void {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.ip_validation) return;
  updateMethod(methodIndex, {
    ip_validation: {
      ...method.ip_validation,
      allowed_ips: method.ip_validation.allowed_ips.filter((_, idx) => idx !== ipIndex)
    }
  });
}

function setAllowedIpField(
  methodIndex: number,
  ipIndex: number,
  field: "name" | "ip",
  value: string
): void {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.ip_validation) return;
  const allowed = method.ip_validation.allowed_ips.map((item, idx) =>
    idx === ipIndex ? { ...item, [field]: value } : item
  );
  updateMethod(methodIndex, { ip_validation: { ...method.ip_validation, allowed_ips: allowed } });
}

// ---- JWT ------------------------------------------------------------------

function setJwtKid(methodIndex: number, value: string): void {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.jwt) return;
  updateMethod(methodIndex, { jwt: { ...method.jwt, kid: value } });
}

function setJwtRoles(methodIndex: number, value: string): void {
  jwtRolesText.value = { ...jwtRolesText.value, [methodIndex]: value };
  const method = localAuth.value.methods[methodIndex];
  if (method?.jwt) {
    updateMethod(methodIndex, { jwt: { ...method.jwt, roles: linesToArray(value) } });
  }
}

function jwtKidSelectOptions(currentKid: string) {
  const options = jwtKidOptions.value.map((kid) => ({ label: kid, value: kid }));
  if (currentKid && !jwtKidOptions.value.includes(currentKid)) {
    options.push({ label: `${currentKid} (current)`, value: currentKid });
  }
  return options;
}
</script>

<template>
  <div class="auth-editor">
    <div class="auth-editor__head">
      <NSwitch :value="localAuth.enabled" @update:value="setEnabled">
        <template #checked>Auth enabled</template>
        <template #unchecked>Auth disabled</template>
      </NSwitch>
      <label v-if="!hideMode" class="auth-editor__mode">
        <span class="field__label">Mode</span>
        <NSelect
          :value="localAuth.mode"
          :options="AUTH_MODE_OPTIONS"
          @update:value="(value: AuthMode) => setMode(value)"
        />
      </label>
    </div>

    <div class="auth-editor__add">
      <NButton
        v-for="item in AUTH_METHOD_TYPES"
        :key="item.type"
        size="small"
        dashed
        @click="addMethod(item.type)"
      >
        + {{ item.label }}
      </NButton>
    </div>

    <p v-if="localAuth.methods.length === 0" class="auth-editor__empty muted">
      No auth methods configured — a request passes when it satisfies any one method.
    </p>

    <div class="auth-editor__list">
      <NCard
        v-for="(method, methodIndex) in localAuth.methods"
        :key="methodKeys[methodIndex] || methodIndex"
        size="small"
        class="auth-method"
      >
        <div class="auth-method__head">
          <div class="auth-method__types">
            <NCheckbox
              :checked="!!method.basic"
              @update:checked="(value: boolean) => toggleType(methodIndex, 'basic', value)"
            >
              Basic
            </NCheckbox>
            <NCheckbox
              :checked="!!method.api_key"
              @update:checked="(value: boolean) => toggleType(methodIndex, 'api_key', value)"
            >
              API Key
            </NCheckbox>
            <NCheckbox
              :checked="!!method.jwt"
              @update:checked="(value: boolean) => toggleType(methodIndex, 'jwt', value)"
            >
              JWT
            </NCheckbox>
            <NCheckbox
              :checked="!!method.ip_validation"
              @update:checked="(value: boolean) => toggleType(methodIndex, 'ip_validation', value)"
            >
              IP Validation
            </NCheckbox>
          </div>
          <NButton
            class="danger-icon-button"
            type="error"
            size="small"
            secondary
            circle
            aria-label="Remove method"
            @click="removeMethod(methodIndex)"
          >
            <NIcon :component="TrashOutline" />
          </NButton>
        </div>

        <div v-if="method.basic" class="auth-block">
          <div class="auth-block__head">
            <strong>Basic users</strong>
            <NButton size="tiny" dashed @click="addBasicUser(methodIndex)">+ User</NButton>
          </div>
          <div
            v-for="(user, userIndex) in method.basic.users"
            :key="userIndex"
            class="auth-row auth-row--generatable"
          >
            <VariableInput
              :model-value="user.username"
              :variables="variableOptions"
              placeholder="username"
              @update:model-value="(value: string) => setBasicField(methodIndex, userIndex, 'username', value)"
            />
            <VariableInput
              :model-value="user.password"
              :variables="variableOptions"
              placeholder="password"
              @update:model-value="(value: string) => setBasicField(methodIndex, userIndex, 'password', value)"
            />
            <NButton
              secondary
              circle
              aria-label="Generate password"
              title="Generate password"
              @click="setBasicField(methodIndex, userIndex, 'password', generateSecret(20))"
            >
              <NIcon :component="RefreshOutline" />
            </NButton>
            <NButton
              class="danger-icon-button"
              type="error"
              secondary
              circle
              aria-label="Remove user"
              @click="removeBasicUser(methodIndex, userIndex)"
            >
              <NIcon :component="TrashOutline" />
            </NButton>
          </div>
        </div>

        <div v-if="method.api_key" class="auth-block">
          <label class="field">
            <span class="field__label">Header</span>
            <VariableInput
              :model-value="method.api_key.header"
              :variables="variableOptions"
              placeholder="X-Api-Key"
              @update:model-value="(value: string) => setApiHeader(methodIndex, value)"
            />
          </label>
          <div class="auth-block__head">
            <strong>Keys</strong>
            <NButton size="tiny" dashed @click="addApiKey(methodIndex)">+ Key</NButton>
          </div>
          <p v-if="method.api_key.keys.length === 0" class="muted auth-block__empty">No keys yet.</p>
          <div
            v-for="(item, keyIndex) in method.api_key.keys"
            :key="keyIndex"
            class="auth-row auth-row--named auth-row--generatable"
          >
            <NInput
              :value="item.name"
              placeholder="name (optional)"
              @update:value="(value: string) => setApiKeyField(methodIndex, keyIndex, 'name', value)"
            />
            <VariableInput
              :model-value="item.key"
              :variables="variableOptions"
              placeholder="key"
              @update:model-value="(value: string) => setApiKeyField(methodIndex, keyIndex, 'key', value)"
            />
            <NButton
              secondary
              circle
              aria-label="Generate key"
              title="Generate key"
              @click="setApiKeyField(methodIndex, keyIndex, 'key', generateSecret(42))"
            >
              <NIcon :component="RefreshOutline" />
            </NButton>
            <NButton
              class="danger-icon-button"
              type="error"
              secondary
              circle
              aria-label="Remove key"
              @click="removeApiKey(methodIndex, keyIndex)"
            >
              <NIcon :component="TrashOutline" />
            </NButton>
          </div>
        </div>

        <div v-if="method.jwt" class="auth-block">
          <label class="field">
            <span class="field__label">KID</span>
            <NSelect
              v-if="jwtKidOptions.length > 0"
              :value="method.jwt.kid"
              clearable
              filterable
              tag
              placeholder="Select or type a KID"
              :options="jwtKidSelectOptions(method.jwt.kid)"
              @update:value="(value: string | null) => setJwtKid(methodIndex, value || '')"
            />
            <NInput
              v-else
              :value="method.jwt.kid"
              placeholder="provider-main-key"
              @update:value="(value: string) => setJwtKid(methodIndex, value)"
            />
          </label>
          <label class="field">
            <span class="field__label">Roles (one per line, optional)</span>
            <VariableInput
              :model-value="jwtRolesText[methodIndex] || ''"
              :variables="variableOptions"
              type="textarea"
              :rows="3"
              @update:model-value="(value: string) => setJwtRoles(methodIndex, value)"
            />
          </label>
        </div>

        <div v-if="method.ip_validation" class="auth-block">
          <div class="auth-block__head">
            <strong>Allowed IPs</strong>
            <NButton size="tiny" dashed @click="addAllowedIp(methodIndex)">+ IP</NButton>
          </div>
          <p v-if="method.ip_validation.allowed_ips.length === 0" class="muted auth-block__empty">
            No IPs yet.
          </p>
          <div
            v-for="(item, ipIndex) in method.ip_validation.allowed_ips"
            :key="ipIndex"
            class="auth-row auth-row--named"
          >
            <NInput
              :value="item.name"
              placeholder="name (optional)"
              @update:value="(value: string) => setAllowedIpField(methodIndex, ipIndex, 'name', value)"
            />
            <VariableInput
              :model-value="item.ip"
              :variables="variableOptions"
              placeholder="IP address or CIDR"
              @update:model-value="(value: string) => setAllowedIpField(methodIndex, ipIndex, 'ip', value)"
            />
            <NButton
              class="danger-icon-button"
              type="error"
              secondary
              circle
              aria-label="Remove IP"
              @click="removeAllowedIp(methodIndex, ipIndex)"
            >
              <NIcon :component="TrashOutline" />
            </NButton>
          </div>
        </div>
      </NCard>
    </div>
  </div>
</template>

<style scoped>
.auth-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.auth-editor__head {
  display: flex;
  align-items: flex-end;
  gap: 18px;
  flex-wrap: wrap;
}

.auth-editor__mode {
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-width: 160px;
}

.auth-editor__add {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.auth-editor__empty {
  margin: 0;
  font-size: 13px;
}

.auth-editor__list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.auth-method__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.auth-method__types {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
}

.auth-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--c-border);
}

.auth-block__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.auth-block__head strong {
  font-size: 13px;
}

.auth-block__empty {
  margin: 0;
  font-size: 12.5px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.field__label {
  font-size: 12px;
  color: var(--c-text-3);
}

.auth-row {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 8px;
  align-items: center;
}

.auth-row--named {
  grid-template-columns: minmax(120px, 0.5fr) 1fr auto;
}

.auth-row--generatable {
  grid-template-columns: 1fr 1fr auto auto;
}

.auth-row--named.auth-row--generatable {
  grid-template-columns: minmax(120px, 0.5fr) 1fr auto auto;
}

@media (max-width: 560px) {
  .auth-row,
  .auth-row--named,
  .auth-row--generatable,
  .auth-row--named.auth-row--generatable {
    grid-template-columns: 1fr auto;
  }
}
</style>
