<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { createApp, getApp, getRoot, getRootJwtKidsByUrls, updateApp } from "../lib/api";
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

const form = ref<AppMain>({
  id: "",
  active: true,
  path_prefix: "",
  name: "",
  backend: {
    url: ""
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

onMounted(() => {
  void Promise.all([load(), loadJwtKidOptions()]);
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
      <input v-model="form.backend.url" placeholder="https://example.com" required />
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
