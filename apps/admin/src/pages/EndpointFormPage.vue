<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { createEndpoint, getApp, getEndpoint, updateEndpoint } from "../lib/api";
import { emptyAuth, parseAuthFromJson, prettyJson } from "../lib/forms";
import { notifyError, notifySuccess } from "../lib/notify";
import type { EndpointMain } from "../types/api";

const route = useRoute();
const router = useRouter();

const isEdit = computed(() => typeof route.params.id === "string" && route.params.id.length > 0);
const endpointId = computed(() => (typeof route.params.id === "string" ? route.params.id : ""));
const appIdFromRoute = computed(() => (typeof route.params.appId === "string" ? route.params.appId : ""));

const loading = ref(false);
const saving = ref(false);
const errorMessage = ref("");
const appName = ref("");

const form = ref<EndpointMain>({
  id: "",
  app_id: appIdFromRoute.value,
  active: true,
  method: "GET",
  path: "",
  backend: {
    custom_path: ""
  },
  auth: {
    ...emptyAuth
  }
});
const authJson = ref(prettyJson(emptyAuth));
const appDisplayName = computed(() => appName.value || form.value.app_id || "-");

async function loadAppName() {
  if (!form.value.app_id) {
    appName.value = "";
    return;
  }
  try {
    const app = await getApp(form.value.app_id);
    appName.value = app.name;
  } catch {
    appName.value = "";
  }
}

async function load() {
  if (!isEdit.value) {
    await loadAppName();
    return;
  }
  loading.value = true;
  errorMessage.value = "";
  try {
    const item = await getEndpoint(endpointId.value);
    form.value = item;
    authJson.value = prettyJson(item.auth || emptyAuth);
    await loadAppName();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load endpoint";
  } finally {
    loading.value = false;
  }
}

async function submit() {
  saving.value = true;
  errorMessage.value = "";
  try {
    form.value.auth = parseAuthFromJson(authJson.value);

    if (isEdit.value) {
      await updateEndpoint(form.value);
      notifySuccess("Endpoint updated");
      await router.push({ name: "app-details", params: { id: form.value.app_id } });
      return;
    }
    const created = await createEndpoint(form.value);
    notifySuccess("Endpoint created");
    await router.push({ name: "endpoint-edit", params: { id: created.id } });
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to save endpoint";
    notifyError(errorMessage.value);
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  void load();
});
</script>

<template>
  <div class="page-header">
    <h2>{{ isEdit ? "Edit Endpoint" : "Create Endpoint" }}</h2>
  </div>

  <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
  <p v-if="loading" class="muted">Loading...</p>

  <form v-else class="stack" @submit.prevent="submit">
    <div class="field">
      <span>Application</span>
      <div class="field-readonly">{{ appDisplayName }}</div>
    </div>
    <label class="field">
      <span>Method</span>
      <input v-model="form.method" placeholder="GET" required />
    </label>
    <label class="field">
      <span>Path</span>
      <input v-model="form.path" placeholder="/users" required />
    </label>
    <label class="field">
      <span>Custom Backend Path</span>
      <input v-model="form.backend.custom_path" placeholder="internal/path" />
    </label>
    <label class="check">
      <input v-model="form.active" type="checkbox" />
      <span>Active</span>
    </label>
    <label class="field">
      <span>Auth JSON</span>
      <textarea v-model="authJson" rows="14" spellcheck="false"></textarea>
    </label>

    <div class="actions">
      <button class="primary-button" type="submit" :disabled="saving">
        {{ saving ? "Saving..." : "Save" }}
      </button>
      <button class="secondary-button" type="button" :disabled="saving" @click="router.back()">Cancel</button>
    </div>
  </form>
</template>
