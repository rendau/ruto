<script setup lang="ts">
import { computed } from "vue";
import { NTag } from "naive-ui";
import { LockClosedOutline, LockOpenOutline } from "@vicons/ionicons5";
import { NIcon } from "naive-ui";
import type { Auth, AuthMethod } from "@/api/types";

const props = defineProps<{ auth: Auth }>();

interface MethodLine {
  type: string;
  detail: string;
}

function describeMethod(method: AuthMethod): MethodLine[] {
  const lines: MethodLine[] = [];
  if (method.basic) {
    const users = method.basic.users.map((u) => u.username).filter(Boolean);
    lines.push({
      type: "Basic",
      detail: users.length ? users.join(", ") : `${method.basic.users.length} user(s)`
    });
  }
  if (method.api_key) {
    lines.push({
      type: "API Key",
      detail: `${method.api_key.header || "header"} · ${method.api_key.keys.length} key(s)`
    });
  }
  if (method.jwt) {
    const roles = method.jwt.roles || [];
    lines.push({
      type: "JWT",
      detail: `${method.jwt.kid || "any kid"}${roles.length ? ` · roles: ${roles.join(", ")}` : ""}`
    });
  }
  if (method.ip_validation) {
    const ips = method.ip_validation.allowed_ips.map((i) => i.ip).filter(Boolean);
    lines.push({ type: "IP", detail: ips.length ? ips.join(", ") : "no IPs" });
  }
  return lines;
}

const methodLines = computed(() => props.auth.methods.flatMap(describeMethod));
</script>

<template>
  <div class="auth-summary">
    <div class="auth-summary__head">
      <NIcon
        :component="auth.enabled ? LockClosedOutline : LockOpenOutline"
        :color="auth.enabled ? 'var(--c-success)' : 'var(--c-text-3)'"
      />
      <span v-if="!auth.enabled" class="muted">Public access — auth disabled</span>
      <template v-else>
        <span class="auth-summary__on">Auth enabled</span>
        <NTag size="tiny" :bordered="false" round>mode: {{ auth.mode }}</NTag>
      </template>
    </div>
    <div v-if="auth.enabled" class="auth-summary__methods">
      <div v-if="methodLines.length === 0" class="muted auth-summary__empty">
        No methods configured.
      </div>
      <div v-for="(line, index) in methodLines" :key="index" class="auth-summary__method">
        <NTag size="tiny" type="primary" :bordered="false">{{ line.type }}</NTag>
        <span class="auth-summary__detail mono">{{ line.detail }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.auth-summary {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.auth-summary__head {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13.5px;
}

.auth-summary__on {
  font-weight: 600;
  color: var(--c-text);
}

.auth-summary__methods {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.auth-summary__method {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.auth-summary__detail {
  font-size: 12.5px;
  color: var(--c-text-2);
  overflow-wrap: anywhere;
}

.auth-summary__empty {
  font-size: 13px;
}
</style>
