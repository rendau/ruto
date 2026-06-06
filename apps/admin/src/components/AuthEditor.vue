<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { TrashOutline } from "@vicons/ionicons5";
import { arrayToLines, cloneAuthMethod, createEmptyAuthMethod, linesToArray, normalizeAuth } from "../lib/forms";
import type { Auth, AuthMethod, Variable } from "../types/api";
import VariableInput from "./VariableInput.vue";

const props = defineProps<{
  modelValue: Auth;
  jwtKidOptions?: string[];
  variableOptions?: Variable[];
}>();

const emit = defineEmits<{
  (event: "update:modelValue", value: Auth): void;
}>();

const localAuth = ref<Auth>(normalizeAuth(props.modelValue));
const methodText = ref<Record<number, { jwtRoles: string }>>({});
const pushingToParent = ref(false);
const methodKeys = ref<string[]>([]);
let nextMethodKey = 0;

const jwtKidOptions = computed(() => {
  const source = props.jwtKidOptions || [];
  return Array.from(new Set(source.map((v) => v.trim()).filter(Boolean))).sort();
});

watch(
  () => props.modelValue,
  (value) => {
    if (pushingToParent.value) {
      pushingToParent.value = false;
      return;
    }
    localAuth.value = normalizeAuth(value);
    syncMethodTextFromAuth();
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

function syncMethodTextFromAuth() {
  const next: Record<number, { jwtRoles: string }> = {};
  localAuth.value.methods.forEach((method, idx) => {
    next[idx] = {
      jwtRoles: arrayToLines(method.jwt?.roles)
    };
  });
  methodText.value = next;
}

function createMethodKey() {
  nextMethodKey += 1;
  return `auth-method-${nextMethodKey}`;
}

function syncMethodKeys() {
  const methodsCount = localAuth.value.methods.length;
  if (methodKeys.value.length < methodsCount) {
    methodKeys.value = [
      ...methodKeys.value,
      ...Array.from({ length: methodsCount - methodKeys.value.length }, () => createMethodKey())
    ];
    return;
  }
  if (methodKeys.value.length > methodsCount) {
    methodKeys.value = methodKeys.value.slice(0, methodsCount);
  }
}

function setEnabled(value: boolean) {
  localAuth.value = {
    ...localAuth.value,
    enabled: value
  };
}

function setMode(value: "extend" | "replace") {
  localAuth.value = {
    ...localAuth.value,
    mode: value
  };
}

function addMethod(type: "basic" | "api_key" | "jwt" | "ip_validation") {
  const nextMethods = [...localAuth.value.methods, createEmptyAuthMethod(type)];
  localAuth.value = {
    ...localAuth.value,
    methods: nextMethods
  };
  syncMethodTextFromAuth();
  syncMethodKeys();
}

function removeMethod(methodIndex: number) {
  const nextMethods = localAuth.value.methods.filter((_, idx) => idx !== methodIndex);
  localAuth.value = {
    ...localAuth.value,
    methods: nextMethods
  };
  methodKeys.value = methodKeys.value.filter((_, idx) => idx !== methodIndex);
  syncMethodTextFromAuth();
  syncMethodKeys();
}

function updateMethod(methodIndex: number, patch: Partial<AuthMethod>) {
  const nextMethods = localAuth.value.methods.map((method, idx) => {
    if (idx !== methodIndex) {
      return method;
    }
    return { ...cloneAuthMethod(method), ...patch };
  });
  localAuth.value = {
    ...localAuth.value,
    methods: nextMethods
  };
}

function toggleType(methodIndex: number, type: "basic" | "api_key" | "jwt" | "ip_validation", enabled: boolean) {
  const method = localAuth.value.methods[methodIndex];
  if (!method) {
    return;
  }
  if (type === "basic") {
    updateMethod(methodIndex, { basic: enabled ? method.basic || { users: [{ username: "", password: "" }] } : undefined });
  } else if (type === "api_key") {
    updateMethod(methodIndex, { api_key: enabled ? method.api_key || { header: "", keys: [] } : undefined });
  } else if (type === "jwt") {
    updateMethod(methodIndex, { jwt: enabled ? method.jwt || { kid: "", roles: [] } : undefined });
  } else {
    updateMethod(methodIndex, { ip_validation: enabled ? method.ip_validation || { allowed_ips: [] } : undefined });
  }
}

function addBasicUser(methodIndex: number) {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.basic) {
    return;
  }
  updateMethod(methodIndex, {
    basic: {
      ...method.basic,
      users: [...method.basic.users, { username: "", password: "" }]
    }
  });
}

function removeBasicUser(methodIndex: number, userIndex: number) {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.basic) {
    return;
  }
  updateMethod(methodIndex, {
    basic: {
      ...method.basic,
      users: method.basic.users.filter((_, idx) => idx !== userIndex)
    }
  });
}

function setBasicUsername(methodIndex: number, userIndex: number, value: string) {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.basic) {
    return;
  }
  const users = method.basic.users.map((user, idx) => (idx === userIndex ? { ...user, username: value } : user));
  updateMethod(methodIndex, { basic: { ...method.basic, users } });
}

function setBasicPassword(methodIndex: number, userIndex: number, value: string) {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.basic) {
    return;
  }
  const users = method.basic.users.map((user, idx) => (idx === userIndex ? { ...user, password: value } : user));
  updateMethod(methodIndex, { basic: { ...method.basic, users } });
}

function setApiHeader(methodIndex: number, value: string) {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.api_key) {
    return;
  }
  updateMethod(methodIndex, { api_key: { ...method.api_key, header: value } });
}

function addApiKey(methodIndex: number) {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.api_key) {
    return;
  }
  updateMethod(methodIndex, {
    api_key: { ...method.api_key, keys: [...method.api_key.keys, { name: "", key: "" }] }
  });
}

function removeApiKey(methodIndex: number, keyIndex: number) {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.api_key) {
    return;
  }
  updateMethod(methodIndex, {
    api_key: { ...method.api_key, keys: method.api_key.keys.filter((_, idx) => idx !== keyIndex) }
  });
}

function setApiKeyField(methodIndex: number, keyIndex: number, field: "name" | "key", value: string) {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.api_key) {
    return;
  }
  const keys = method.api_key.keys.map((item, idx) => (idx === keyIndex ? { ...item, [field]: value } : item));
  updateMethod(methodIndex, { api_key: { ...method.api_key, keys } });
}

function addAllowedIp(methodIndex: number) {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.ip_validation) {
    return;
  }
  updateMethod(methodIndex, {
    ip_validation: { ...method.ip_validation, allowed_ips: [...method.ip_validation.allowed_ips, { name: "", ip: "" }] }
  });
}

function removeAllowedIp(methodIndex: number, ipIndex: number) {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.ip_validation) {
    return;
  }
  updateMethod(methodIndex, {
    ip_validation: { ...method.ip_validation, allowed_ips: method.ip_validation.allowed_ips.filter((_, idx) => idx !== ipIndex) }
  });
}

function setAllowedIpField(methodIndex: number, ipIndex: number, field: "name" | "ip", value: string) {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.ip_validation) {
    return;
  }
  const allowed_ips = method.ip_validation.allowed_ips.map((item, idx) =>
    idx === ipIndex ? { ...item, [field]: value } : item
  );
  updateMethod(methodIndex, { ip_validation: { ...method.ip_validation, allowed_ips } });
}

function setJwtKid(methodIndex: number, value: string) {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.jwt) {
    return;
  }
  updateMethod(methodIndex, { jwt: { ...method.jwt, kid: value } });
}

function setJwtRoles(methodIndex: number, value: string) {
  const prev = methodText.value[methodIndex] || { jwtRoles: "" };
  methodText.value = {
    ...methodText.value,
    [methodIndex]: { ...prev, jwtRoles: value }
  };
  const method = localAuth.value.methods[methodIndex];
  if (method?.jwt) {
    updateMethod(methodIndex, { jwt: { ...method.jwt, roles: linesToArray(value) } });
  }
}
</script>

<template>
  <div class="auth-editor">
    <div class="auth-head">
      <n-switch :value="localAuth.enabled" @update:value="setEnabled">
        <template #checked>Enabled</template>
        <template #unchecked>Disabled</template>
      </n-switch>
      <n-select
        class="auth-mode-select"
        :value="localAuth.mode"
        :options="[
          { label: 'extend', value: 'extend' },
          { label: 'replace', value: 'replace' }
        ]"
        @update:value="(value: string) => setMode(value === 'replace' ? 'replace' : 'extend')"
      />
    </div>

    <n-space class="auth-method-add">
      <n-button size="small" secondary @click="addMethod('basic')">+ Basic</n-button>
      <n-button size="small" secondary @click="addMethod('api_key')">+ API Key</n-button>
      <n-button size="small" secondary @click="addMethod('jwt')">+ JWT</n-button>
      <n-button size="small" secondary @click="addMethod('ip_validation')">+ IP Validation</n-button>
    </n-space>

    <p v-if="localAuth.methods.length === 0" class="auth-empty muted">No auth methods configured.</p>

    <TransitionGroup name="auth-method" tag="div" class="auth-method-list">
      <n-card v-for="(method, methodIndex) in localAuth.methods" :key="methodKeys[methodIndex] || methodIndex" class="auth-method-card" size="small">
        <div class="auth-method-card-head">
          <div class="auth-type-toggle">
            <n-checkbox :checked="!!method.basic" @update:checked="(value: boolean) => toggleType(methodIndex, 'basic', value)">Basic</n-checkbox>
            <n-checkbox :checked="!!method.api_key" @update:checked="(value: boolean) => toggleType(methodIndex, 'api_key', value)">API Key</n-checkbox>
            <n-checkbox :checked="!!method.jwt" @update:checked="(value: boolean) => toggleType(methodIndex, 'jwt', value)">JWT</n-checkbox>
            <n-checkbox :checked="!!method.ip_validation" @update:checked="(value: boolean) => toggleType(methodIndex, 'ip_validation', value)">IP Validation</n-checkbox>
          </div>
          <n-button
            class="danger-icon-button auth-method-remove"
            type="error"
            size="small"
            secondary
            circle
            aria-label="Remove method"
            title="Remove method"
            @click="removeMethod(methodIndex)"
          >
            <n-icon :component="TrashOutline" />
          </n-button>
        </div>

      <div v-if="method.basic" class="auth-type-block">
        <div class="field-inline">
          <strong>Basic Users</strong>
          <n-button size="small" secondary @click="addBasicUser(methodIndex)">+ User</n-button>
        </div>
        <div v-for="(user, userIndex) in method.basic.users" :key="userIndex" class="auth-basic-user-row">
          <VariableInput
            :model-value="user.username"
            :variables="variableOptions"
            placeholder="username"
            @update:model-value="(value: string) => setBasicUsername(methodIndex, userIndex, value)"
          />
          <VariableInput
            :model-value="user.password"
            :variables="variableOptions"
            placeholder="password"
            type="password"
            @update:model-value="(value: string) => setBasicPassword(methodIndex, userIndex, value)"
          />
          <n-button
            class="danger-icon-button"
            type="error"
            secondary
            circle
            aria-label="Remove basic user"
            title="Remove basic user"
            @click="removeBasicUser(methodIndex, userIndex)"
          >
            <n-icon :component="TrashOutline" />
          </n-button>
        </div>
      </div>

      <div v-if="method.api_key" class="auth-type-block">
        <label class="field">
          <span>Header</span>
          <VariableInput
            :model-value="method.api_key.header"
            :variables="variableOptions"
            placeholder="X-Api-Key"
            @update:model-value="(value: string) => setApiHeader(methodIndex, value)"
          />
        </label>
        <div class="field">
          <div class="field-inline">
            <span>Keys</span>
            <n-button size="small" secondary @click="addApiKey(methodIndex)">+ Key</n-button>
          </div>
          <p v-if="method.api_key.keys.length === 0" class="muted auth-named-empty">No keys yet.</p>
          <div v-for="(item, keyIndex) in method.api_key.keys" :key="keyIndex" class="auth-named-row">
            <n-input
              class="auth-named-name"
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
            <n-button
              class="danger-icon-button"
              type="error"
              secondary
              circle
              aria-label="Remove key"
              title="Remove key"
              @click="removeApiKey(methodIndex, keyIndex)"
            >
              <n-icon :component="TrashOutline" />
            </n-button>
          </div>
        </div>
      </div>

      <div v-if="method.jwt" class="auth-type-block">
        <label class="field">
          <span>KID</span>
          <n-select
            v-if="jwtKidOptions.length > 0"
            :value="method.jwt.kid"
            clearable
            placeholder="Select KID"
            :options="[
              ...jwtKidOptions.map((kid) => ({ label: kid, value: kid })),
              ...(method.jwt.kid && !jwtKidOptions.includes(method.jwt.kid) ? [{ label: `${method.jwt.kid} (current)`, value: method.jwt.kid }] : [])
            ]"
            @update:value="(value: string | null) => setJwtKid(methodIndex, value || '')"
          />
          <n-input
            v-else
            :value="method.jwt.kid"
            placeholder="provider-main-key"
            @update:value="(value: string) => setJwtKid(methodIndex, value)"
          />
        </label>
        <label class="field">
          <span>Roles (one per line)</span>
          <VariableInput
            :model-value="methodText[methodIndex]?.jwtRoles || ''"
            :variables="variableOptions"
            type="textarea"
            :rows="3"
            @update:model-value="(value: string) => setJwtRoles(methodIndex, value)"
          />
        </label>
      </div>

      <div v-if="method.ip_validation" class="auth-type-block">
        <div class="field">
          <div class="field-inline">
            <span>Allowed IPs</span>
            <n-button size="small" secondary @click="addAllowedIp(methodIndex)">+ IP</n-button>
          </div>
          <p v-if="method.ip_validation.allowed_ips.length === 0" class="muted auth-named-empty">No IPs yet.</p>
          <div v-for="(item, ipIndex) in method.ip_validation.allowed_ips" :key="ipIndex" class="auth-named-row">
            <n-input
              class="auth-named-name"
              :value="item.name"
              placeholder="name (optional)"
              @update:value="(value: string) => setAllowedIpField(methodIndex, ipIndex, 'name', value)"
            />
            <VariableInput
              :model-value="item.ip"
              :variables="variableOptions"
              placeholder="IP address"
              @update:model-value="(value: string) => setAllowedIpField(methodIndex, ipIndex, 'ip', value)"
            />
            <n-button
              class="danger-icon-button"
              type="error"
              secondary
              circle
              aria-label="Remove IP"
              title="Remove IP"
              @click="removeAllowedIp(methodIndex, ipIndex)"
            >
              <n-icon :component="TrashOutline" />
            </n-button>
          </div>
        </div>
      </div>
      </n-card>
    </TransitionGroup>
  </div>
</template>
