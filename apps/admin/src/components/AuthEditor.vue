<script setup lang="ts">
import { computed, ref, watch } from "vue";
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
      <label class="check auth-control">
        <input :checked="localAuth.enabled" type="checkbox" @change="setEnabled(($event.target as HTMLInputElement).checked)" />
        <span>Enabled</span>
      </label>
      <label class="auth-control auth-mode-control">
        <span class="auth-control-label">Mode</span>
        <select :value="localAuth.mode" @change="setMode(($event.target as HTMLSelectElement).value === 'replace' ? 'replace' : 'extend')">
          <option value="extend">extend</option>
          <option value="replace">replace</option>
        </select>
      </label>
    </div>

    <div class="auth-method-add">
      <button class="secondary-button" type="button" @click="addMethod('basic')">+ Basic</button>
      <button class="secondary-button" type="button" @click="addMethod('api_key')">+ API Key</button>
      <button class="secondary-button" type="button" @click="addMethod('jwt')">+ JWT</button>
      <button class="secondary-button" type="button" @click="addMethod('ip_validation')">+ IP Validation</button>
    </div>

    <p v-if="localAuth.methods.length === 0" class="auth-empty muted">No auth methods configured.</p>

    <TransitionGroup name="auth-method" tag="div" class="auth-method-list">
      <div v-for="(method, methodIndex) in localAuth.methods" :key="methodKeys[methodIndex] || methodIndex" class="auth-method-card">
        <div class="auth-method-card-head">
          <div class="auth-type-toggle">
            <label class="check">
              <input :checked="!!method.basic" type="checkbox" @change="toggleType(methodIndex, 'basic', ($event.target as HTMLInputElement).checked)" />
              <span>Basic</span>
            </label>
            <label class="check">
              <input :checked="!!method.api_key" type="checkbox" @change="toggleType(methodIndex, 'api_key', ($event.target as HTMLInputElement).checked)" />
              <span>API Key</span>
            </label>
            <label class="check">
              <input :checked="!!method.jwt" type="checkbox" @change="toggleType(methodIndex, 'jwt', ($event.target as HTMLInputElement).checked)" />
              <span>JWT</span>
            </label>
            <label class="check">
              <input :checked="!!method.ip_validation" type="checkbox" @change="toggleType(methodIndex, 'ip_validation', ($event.target as HTMLInputElement).checked)" />
              <span>IP Validation</span>
            </label>
          </div>
          <button class="icon-action-button danger" type="button" aria-label="Remove method" title="Remove method" @click="removeMethod(methodIndex)">
            <span class="icon-action-glyph">×</span>
          </button>
        </div>

      <div v-if="method.basic" class="auth-type-block">
        <div class="field-inline">
          <strong>Basic Users</strong>
          <button class="secondary-button" type="button" @click="addBasicUser(methodIndex)">+ User</button>
        </div>
        <div v-for="(user, userIndex) in method.basic.users" :key="userIndex" class="auth-basic-user-row">
          <input :value="user.username" placeholder="username" @input="setBasicUsername(methodIndex, userIndex, ($event.target as HTMLInputElement).value)" />
          <input
            :value="user.password"
            placeholder="password"
            type="password"
            @input="setBasicPassword(methodIndex, userIndex, ($event.target as HTMLInputElement).value)"
          />
          <button class="danger-text-button" type="button" @click="removeBasicUser(methodIndex, userIndex)">×</button>
        </div>
      </div>

      <div v-if="method.api_key" class="auth-type-block">
        <label class="field">
          <span>Header</span>
          <input :value="method.api_key.header" placeholder="X-Api-Key" @input="setApiHeader(methodIndex, ($event.target as HTMLInputElement).value)" />
        </label>
        <label class="field">
          <span>Keys (one per line)</span>
          <textarea
            :value="methodText[methodIndex]?.apiKeys || ''"
            rows="3"
            @input="setMethodText(methodIndex, 'apiKeys', ($event.target as HTMLTextAreaElement).value)"
          ></textarea>
        </label>
      </div>

      <div v-if="method.jwt" class="auth-type-block">
        <label class="field">
          <span>KID</span>
          <select v-if="jwtKidOptions.length > 0" :value="method.jwt.kid" @change="setJwtKid(methodIndex, ($event.target as HTMLSelectElement).value)">
            <option value="" disabled>Select KID</option>
            <option v-for="kid in jwtKidOptions" :key="kid" :value="kid">{{ kid }}</option>
            <option v-if="method.jwt.kid && !jwtKidOptions.includes(method.jwt.kid)" :value="method.jwt.kid">{{ method.jwt.kid }} (current)</option>
          </select>
          <input
            v-else
            :value="method.jwt.kid"
            placeholder="provider-main-key"
            @input="setJwtKid(methodIndex, ($event.target as HTMLInputElement).value)"
          />
        </label>
        <label class="field">
          <span>Roles (one per line)</span>
          <textarea
            :value="methodText[methodIndex]?.jwtRoles || ''"
            rows="3"
            @input="setMethodText(methodIndex, 'jwtRoles', ($event.target as HTMLTextAreaElement).value)"
          ></textarea>
        </label>
      </div>

      <div v-if="method.ip_validation" class="auth-type-block">
        <label class="field">
          <span>Allowed IPs (one per line)</span>
          <textarea
            :value="methodText[methodIndex]?.allowedIps || ''"
            rows="3"
            @input="setMethodText(methodIndex, 'allowedIps', ($event.target as HTMLTextAreaElement).value)"
          ></textarea>
        </label>
      </div>
      </div>
    </TransitionGroup>
  </div>
</template>
