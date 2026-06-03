<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { TrashOutline } from "@vicons/ionicons5";
import { arrayToLines, cloneAuthMethod, createEmptyAuthMethod, linesToArray, normalizeAuth } from "../lib/forms";
import type { Auth, AuthMethod } from "../types/api";

const props = defineProps<{
  modelValue: Auth;
  jwtKidOptions?: string[];
}>();

const emit = defineEmits<{
  (event: "update:modelValue", value: Auth): void;
}>();

const localAuth = ref<Auth>(normalizeAuth(props.modelValue));
const methodText = ref<
  Record<number, { apiKeys: string; jwtRoles: string; allowedIps: string }>
>({});
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
  const next: Record<number, { apiKeys: string; jwtRoles: string; allowedIps: string }> = {};
  localAuth.value.methods.forEach((method, idx) => {
    next[idx] = {
      apiKeys: arrayToLines(method.api_key?.keys),
      jwtRoles: arrayToLines(method.jwt?.roles),
      allowedIps: arrayToLines(method.ip_validation?.allowed_ips)
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

function setJwtKid(methodIndex: number, value: string) {
  const method = localAuth.value.methods[methodIndex];
  if (!method?.jwt) {
    return;
  }
  updateMethod(methodIndex, { jwt: { ...method.jwt, kid: value } });
}

function setMethodText(methodIndex: number, field: "apiKeys" | "jwtRoles" | "allowedIps", value: string) {
  const prev = methodText.value[methodIndex] || { apiKeys: "", jwtRoles: "", allowedIps: "" };
  methodText.value = {
    ...methodText.value,
    [methodIndex]: {
      ...prev,
      [field]: value
    }
  };
  const method = localAuth.value.methods[methodIndex];
  if (!method) {
    return;
  }
  if (field === "apiKeys" && method.api_key) {
    updateMethod(methodIndex, { api_key: { ...method.api_key, keys: linesToArray(value) } });
  } else if (field === "jwtRoles" && method.jwt) {
    updateMethod(methodIndex, { jwt: { ...method.jwt, roles: linesToArray(value) } });
  } else if (field === "allowedIps" && method.ip_validation) {
    updateMethod(methodIndex, { ip_validation: { ...method.ip_validation, allowed_ips: linesToArray(value) } });
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
          <n-input :value="user.username" placeholder="username" @update:value="(value: string) => setBasicUsername(methodIndex, userIndex, value)" />
          <n-input
            :value="user.password"
            placeholder="password"
            type="password"
            @update:value="(value: string) => setBasicPassword(methodIndex, userIndex, value)"
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
          <n-input :value="method.api_key.header" placeholder="X-Api-Key" @update:value="(value: string) => setApiHeader(methodIndex, value)" />
        </label>
        <label class="field">
          <span>Keys (one per line)</span>
          <n-input
            :value="methodText[methodIndex]?.apiKeys || ''"
            type="textarea"
            rows="3"
            @update:value="(value: string) => setMethodText(methodIndex, 'apiKeys', value)"
          />
        </label>
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
          <n-input
            :value="methodText[methodIndex]?.jwtRoles || ''"
            type="textarea"
            rows="3"
            @update:value="(value: string) => setMethodText(methodIndex, 'jwtRoles', value)"
          />
        </label>
      </div>

      <div v-if="method.ip_validation" class="auth-type-block">
        <label class="field">
          <span>Allowed IPs (one per line)</span>
          <n-input
            :value="methodText[methodIndex]?.allowedIps || ''"
            type="textarea"
            rows="3"
            @update:value="(value: string) => setMethodText(methodIndex, 'allowedIps', value)"
          />
        </label>
      </div>
      </n-card>
    </TransitionGroup>
  </div>
</template>
