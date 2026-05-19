<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { createEndpoint, getApp, getEndpoint, getRootJwtKids, updateEndpoint } from "../lib/api";
import AuthEditor from "../components/AuthEditor.vue";
import { emptyAuth, normalizeAuth } from "../lib/forms";
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
const jwtKidOptions = ref<string[]>([]);

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
    form.value = {
      ...item,
      auth: normalizeAuth(item.auth)
    };
    await loadAppName();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load endpoint";
  } finally {
    loading.value = false;
  }
}

async function loadJwtKidOptions() {
  try {
    const rep = await getRootJwtKids();
    jwtKidOptions.value = rep.kids || [];
  } catch {
    jwtKidOptions.value = [];
  }
}

async function submit() {
  saving.value = true;
  errorMessage.value = "";
  try {
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
  void Promise.all([load(), loadJwtKidOptions()]);
});
</script>

<template>
  <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
  <p v-if="loading" class="muted">Loading...</p>

  <form v-else class="stack" @submit.prevent="submit">
    <div class="field">
      <span>Application</span>
      <div class="field-readonly">{{ appDisplayName }}</div>
    </div>
    <label class="check">
      <input v-model="form.active" type="checkbox" />
      <span>Active</span>
    </label>
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
    <div class="field">
      <span>Auth</span>
      <AuthEditor v-model="form.auth" :jwt-kid-options="jwtKidOptions" />
    </div>

    <div class="actions">
      <button class="primary-button" type="submit" :disabled="saving">
        {{ saving ? "Saving..." : "Save" }}
      </button>
      <button class="secondary-button" type="button" :disabled="saving" @click="router.back()">Cancel</button>
    </div>
  </form>
</template>
