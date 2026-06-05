<script setup lang="ts">
import { computed, type Component } from "vue";
import { GlobeOutline, KeyOutline, ListOutline, PersonOutline } from "@vicons/ionicons5";
import type { Auth, AuthMethod } from "../types/api";

type MethodField = {
  label: string;
  value: string;
  multiline?: boolean;
};

type MethodTypeView = {
  key: string;
  label: string;
  icon: Component;
  fields: MethodField[];
};

type MethodView = {
  key: string;
  title: string;
  types: MethodTypeView[];
};

const props = defineProps<{
  auth?: Auth | null;
}>();

function listValue(values: string[] | undefined): string {
  if (!values || values.length === 0) {
    return "none";
  }
  return values.join("\n");
}

function methodTypes(method: AuthMethod): MethodTypeView[] {
  const views: MethodTypeView[] = [];
  if (method.ip_validation) {
    views.push({
      key: "ip",
      label: "IP Validation",
      icon: GlobeOutline,
      fields: [
        {
          label: "Allowed IPs",
          value: listValue(method.ip_validation.allowed_ips),
          multiline: true
        }
      ]
    });
  }

  if (method.jwt) {
    views.push({
      key: "jwt",
      label: "JWT",
      icon: KeyOutline,
      fields: [
        { label: "KID", value: method.jwt.kid || "-" },
        {
          label: "Roles",
          value: listValue(method.jwt.roles),
          multiline: true
        }
      ]
    });
  }

  if (method.basic) {
    const users = method.basic.users || [];
    const usersValue = users.length > 0
      ? users.map((user, index) => `#${index + 1}\nusername: ${user.username || "-"}\npassword: ${user.password || "-"}`).join("\n\n")
      : "none";

    views.push({
      key: "basic",
      label: "Basic",
      icon: PersonOutline,
      fields: [
        {
          label: "Users",
          value: usersValue,
          multiline: true
        }
      ]
    });
  }

  if (method.api_key) {
    views.push({
      key: "api_key",
      label: "API Key",
      icon: KeyOutline,
      fields: [
        { label: "Header", value: method.api_key.header || "-" },
        {
          label: "Keys",
          value: listValue(method.api_key.keys),
          multiline: true
        }
      ]
    });
  }

  return views;
}

const methodViews = computed(() => {
  const methods = props.auth?.methods || [];
  return methods.map((method, index) => ({
    key: `method-${index}`,
    title: `Method ${index + 1}`,
    types: methodTypes(method)
  })).filter((method) => method.types.length > 0);
});
</script>

<template>
  <div v-if="methodViews.length > 0" class="endpoint-auth-method-list">
    <div v-for="method in methodViews" :key="method.key" class="endpoint-auth-method-row">
      <p class="endpoint-auth-method-title">
        <n-icon :component="ListOutline" />
        <span>{{ method.title }}</span>
      </p>
      <div v-for="typeView in method.types" :key="`${method.key}-${typeView.key}`" class="endpoint-auth-method-type">
        <div class="endpoint-auth-method-label">
          <n-icon :component="typeView.icon" />
          <span>{{ typeView.label }}</span>
        </div>

        <div class="endpoint-auth-fields">
          <div v-for="field in typeView.fields" :key="`${method.key}-${typeView.key}-${field.label}`" class="endpoint-auth-field">
            <span class="label">{{ field.label }}</span>
            <strong :class="{ 'endpoint-auth-field-multiline': field.multiline }">{{ field.value }}</strong>
          </div>
        </div>
      </div>
    </div>
  </div>
  <p v-else class="muted">No auth methods configured.</p>
</template>

<style scoped>
.endpoint-auth-method-list {
  display: grid;
  gap: 8px;
}

.endpoint-auth-method-row {
  display: grid;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid #3f587c;
  border-radius: 6px;
  background: #17263b;
}

.endpoint-auth-method-title {
  margin: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 700;
  color: #b9cde8;
  text-transform: uppercase;
}

.endpoint-auth-method-type {
  display: grid;
  gap: 6px;
  padding: 8px;
  border: 1px solid #324a6b;
  border-radius: 6px;
  background: #1b2d45;
}

.endpoint-auth-method-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 700;
  color: #dce7f8;
}

.endpoint-auth-fields {
  display: grid;
  gap: 6px;
}

.endpoint-auth-field {
  display: grid;
  gap: 2px;
}

.endpoint-auth-field strong {
  color: #dce7f8;
  font-size: 12px;
  overflow-wrap: anywhere;
}

.endpoint-auth-field-multiline {
  white-space: pre-wrap;
}

@media (max-width: 640px) {
  .endpoint-auth-method-type {
    padding: 7px;
  }
}
</style>
