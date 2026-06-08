<script setup lang="ts">
import { computed, type Component } from "vue";
import { GlobeOutline, KeyOutline, LockClosedOutline, PersonOutline } from "@vicons/ionicons5";
import AuthMethodList from "./AuthMethodList.vue";
import type { Auth } from "../types/api";

type AuthIcon = {
  key: string;
  component: Component;
  label: string;
};

const props = withDefaults(defineProps<{
  auth?: Auth | null;
  title?: string;
  hideMode?: boolean;
}>(), {
  title: "Auth"
});

const authSummary = computed(() => {
  if (!props.auth?.enabled) {
    return "Public access (auth disabled)";
  }
  if (props.hideMode) {
    return "Auth enabled";
  }
  const mode = (props.auth.mode || "extend").toLowerCase() === "replace" ? "replace" : "extend";
  return `Auth enabled, mode: ${mode}`;
});

const authIcons = computed<AuthIcon[]>(() => {
  const methods = props.auth?.methods || [];
  const hasIpValidation = methods.some((method) => Boolean(method.ip_validation));
  const hasJwt = methods.some((method) => Boolean(method.jwt));
  const hasBasic = methods.some((method) => Boolean(method.basic));
  const hasApiKey = methods.some((method) => Boolean(method.api_key));

  const icons: AuthIcon[] = [];
  if (hasIpValidation) {
    icons.push({ key: "ip_validation", component: GlobeOutline, label: "IP Validation" });
  }
  if (hasJwt) {
    icons.push({ key: "jwt", component: KeyOutline, label: "JWT" });
  }
  if (hasBasic) {
    icons.push({ key: "basic", component: PersonOutline, label: "Basic Auth" });
  }
  if (hasApiKey) {
    icons.push({ key: "api_key", component: KeyOutline, label: "API Key" });
  }
  return icons;
});
</script>

<template>
  <div class="auth-card">
    <div class="auth-card-head">
      <h3>{{ title }}</h3>
      <span v-if="auth?.enabled" class="auth-lock-chip" title="Auth required" aria-label="Auth required">
        <n-icon :component="LockClosedOutline" />
      </span>
    </div>

    <p class="muted auth-card-summary">{{ authSummary }}</p>

    <div v-if="authIcons.length > 0" class="auth-card-icons">
      <span
        v-for="authIcon in authIcons"
        :key="authIcon.key"
        class="auth-card-icon"
        :title="authIcon.label"
        :aria-label="authIcon.label"
      >
        <n-icon :component="authIcon.component" />
      </span>
    </div>

    <AuthMethodList :auth="auth" />
  </div>
</template>

<style scoped>
.auth-card {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #3f587c;
}

.auth-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 6px;
}

.auth-card-summary {
  margin-top: 0;
  margin-bottom: 8px;
}

.auth-card-icons {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 8px;
}

.auth-card-icon {
  min-width: 28px;
  height: 24px;
  padding: 0 7px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #4d6487;
  border-radius: 6px;
  background: #2a3f5f;
  color: #dce7f8;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
}
</style>
