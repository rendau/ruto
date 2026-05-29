<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { createApp, getApp, getAppSwaggerUrlByBackendUrl, getRoot, getRootJwtKidsByUrls, updateApp } from "../lib/api";
import AuthEditor from "../components/AuthEditor.vue";
import { normalizeAuth } from "../lib/forms";
import { notifyError, notifySuccess } from "../lib/notify";
import type { AppMain } from "../types/api";
import { useAppsStore } from "../stores/apps";

const route = useRoute();
const router = useRouter();
const appsStore = useAppsStore();

const isEdit = computed(() => typeof route.params.id === "string" && route.params.id.length > 0);
const entityId = computed(() => (typeof route.params.id === "string" ? route.params.id : ""));

const loading = ref(false);
const saving = ref(false);
const errorMessage = ref("");
const jwtKidOptions = ref<string[]>([]);
const discoveringSwagger = ref(false);
const autoDetectedSwaggerUrl = ref("");
let discoverTimer: ReturnType<typeof setTimeout> | null = null;
let discoverRequestSeq = 0;

const form = ref<AppMain>({
  id: "",
  active: true,
  path_prefix: "",
  name: "",
  backend: {
    url: "",
    swagger_url: ""
  },
  auth: {
    enabled: true,
    mode: "extend",
    methods: []
  }
});

async function load() {
  if (!isEdit.value) {
    return;
  }

  loading.value = true;
  errorMessage.value = "";
  try {
    const item = await getApp(entityId.value);
    form.value = {
      ...item,
      auth: normalizeAuth(item.auth)
    };
    if (!form.value.backend.swagger_url.trim()) {
      void discoverSwaggerUrl(form.value.backend.url);
    }
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load app";
  } finally {
    loading.value = false;
  }
}

async function loadJwtKidOptions() {
  try {
    const root = await getRoot();
    const rep = await getRootJwtKidsByUrls({
      urls: (root.jwt || []).map((item) => item.jwk_url).filter(Boolean)
    });
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
      await updateApp(form.value);
      await appsStore.loadMenuApps();
      notifySuccess("Application updated");
      await router.push({ name: "app-details", params: { id: form.value.id } });
      return;
    }

    const created = await createApp(form.value);
    await appsStore.loadMenuApps();
    notifySuccess("Application created");
    await router.push({ name: "app-details", params: { id: created.id } });
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to save app";
    notifyError(errorMessage.value);
  } finally {
    saving.value = false;
  }
}

function onSwaggerUrlInput() {
  if (form.value.backend.swagger_url.trim() !== autoDetectedSwaggerUrl.value) {
    autoDetectedSwaggerUrl.value = "";
  }
}

async function discoverSwaggerUrl(backendUrl: string) {
  const normalizedBackendUrl = backendUrl.trim();
  if (!normalizedBackendUrl) {
    discoveringSwagger.value = false;
    return;
  }

  const requestId = ++discoverRequestSeq;
  discoveringSwagger.value = true;

  try {
    const rep = await getAppSwaggerUrlByBackendUrl({ backend_url: normalizedBackendUrl });
    if (requestId != discoverRequestSeq) {
      return;
    }

    const foundSwaggerUrl = (rep.swagger_url || "").trim();
    if (!foundSwaggerUrl) {
      return;
    }

    const currentSwaggerUrl = form.value.backend.swagger_url.trim();
    if (currentSwaggerUrl && currentSwaggerUrl !== autoDetectedSwaggerUrl.value) {
      return;
    }

    form.value.backend.swagger_url = foundSwaggerUrl;
    autoDetectedSwaggerUrl.value = foundSwaggerUrl;
  } catch {
    // user can still input swagger URL manually
  } finally {
    if (requestId == discoverRequestSeq) {
      discoveringSwagger.value = false;
    }
  }
}

function queueSwaggerDiscovery() {
  if (isEdit.value) {
    return;
  }
  if (discoverTimer) {
    clearTimeout(discoverTimer);
  }
  discoverTimer = setTimeout(() => {
    void discoverSwaggerUrl(form.value.backend.url);
  }, 600);
}

function triggerSwaggerDiscoveryNow() {
  if (isEdit.value) {
    return;
  }
  if (discoverTimer) {
    clearTimeout(discoverTimer);
    discoverTimer = null;
  }
  void discoverSwaggerUrl(form.value.backend.url);
}

onMounted(() => {
  void Promise.all([load(), loadJwtKidOptions()]);
});

onBeforeUnmount(() => {
  if (discoverTimer) {
    clearTimeout(discoverTimer);
  }
  discoverRequestSeq++;
});
</script>

<template>
  <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
  <p v-if="loading" class="muted">Loading...</p>

  <form v-else class="stack" @submit.prevent="submit">
    <label class="check">
      <input v-model="form.active" type="checkbox" />
      <span>Active</span>
    </label>
    <label class="field">
      <span>Name</span>
      <input v-model="form.name" required />
    </label>
    <label class="field">
      <span>Path Prefix</span>
      <input v-model="form.path_prefix" placeholder="/example" required />
    </label>
    <label class="field">
      <span>Backend URL</span>
      <input
        v-model="form.backend.url"
        placeholder="https://example.com"
        required
        @input="queueSwaggerDiscovery"
        @blur="triggerSwaggerDiscoveryNow"
      />
    </label>
    <label class="field">
      <span>Swagger URL</span>
      <div class="input-with-indicator">
        <input
          v-model="form.backend.swagger_url"
          class="swagger-input"
          placeholder="https://example.com/swagger.json"
          @input="onSwaggerUrlInput"
        />
        <span v-if="discoveringSwagger" class="inline-spinner" role="status" aria-label="Searching swagger URL" />
      </div>
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

<style scoped>
.input-with-indicator {
  position: relative;
}

.inline-spinner {
  position: absolute;
  top: 50%;
  right: 12px;
  transform: translateY(-50%);
  width: 14px;
  height: 14px;
  border-radius: 999px;
  border: 2px solid #7c8ba5;
  border-top-color: #dce7f8;
  animation: spin 0.7s linear infinite;
  pointer-events: none;
}

.swagger-input {
  padding-right: 36px;
}

@keyframes spin {
  to {
    transform: translateY(-50%) rotate(360deg);
  }
}
</style>
