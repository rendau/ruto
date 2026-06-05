<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import AuthEditor from "../components/AuthEditor.vue";
import AuthCard from "../components/AuthCard.vue";
import VariableEditor from "../components/VariableEditor.vue";
import { getRoot, getRootInterpolate, getRootJwtKidsByUrls, setRoot } from "../lib/api";
import { arrayToLines, emptyAuth, hasDuplicateVariableKeys, linesToArray, normalizeAuth, normalizeVariables } from "../lib/forms";
import { notifyError, notifySuccess } from "../lib/notify";
import { useAuthStore } from "../stores/auth";
import type { RootMain, Variable } from "../types/api";

const authStore = useAuthStore();
const canEdit = computed(() => Boolean(authStore.profile?.is_admin));

const loading = ref(false);
const saving = ref(false);
const errorMessage = ref("");

const form = ref<RootMain>({
  base_url: "",
  cors: {
    enabled: false,
    allow_credentials: false,
    max_age: "864000",
    allow_origins: ["*"],
    allow_methods: ["*"],
    allow_headers: ["*"]
  },
  jwt: [],
  auth: { ...emptyAuth },
  variables: []
});

const allowOriginsText = ref("*");
const allowMethodsText = ref("*");
const allowHeadersText = ref("*");
const jwkUrlsText = ref("");
const jwtKidOptions = ref<string[]>([]);
const effectiveVariables = ref<Variable[]>([]);
let variablesRequestSeq = 0;

async function load() {
  loading.value = true;
  errorMessage.value = "";
  try {
    const root = await getRoot();
    const kidsRep = await getRootJwtKidsByUrls({
      urls: (root.jwt || []).map((x) => x.jwk_url).filter(Boolean)
    }).catch(() => ({ kids: [] }));
    form.value = {
      ...root,
      auth: normalizeAuth(root.auth),
      variables: root.variables || []
    };
    jwtKidOptions.value = kidsRep.kids || [];
    allowOriginsText.value = arrayToLines(root.cors?.allow_origins);
    allowMethodsText.value = arrayToLines(root.cors?.allow_methods);
    allowHeadersText.value = arrayToLines(root.cors?.allow_headers);
    jwkUrlsText.value = arrayToLines((root.jwt || []).map((x) => x.jwk_url));
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load root settings";
  } finally {
    loading.value = false;
  }
}

async function submit() {
  saving.value = true;
  errorMessage.value = "";
  try {
    if (hasDuplicateVariableKeys(form.value.variables)) {
      throw new Error("Variable keys must be unique");
    }
    form.value.cors.allow_origins = linesToArray(allowOriginsText.value);
    form.value.cors.allow_methods = linesToArray(allowMethodsText.value);
    form.value.cors.allow_headers = linesToArray(allowHeadersText.value);
    form.value.jwt = linesToArray(jwkUrlsText.value).map((jwk_url) => ({ jwk_url }));

    await setRoot(form.value);
    notifySuccess("Root settings updated");
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to save root settings";
    notifyError(errorMessage.value);
  } finally {
    saving.value = false;
  }
}

async function refreshEffectiveVariables() {
  const requestId = ++variablesRequestSeq;
  try {
    const rep = await getRootInterpolate({ variables: form.value.variables || [] });
    if (requestId === variablesRequestSeq) {
      effectiveVariables.value = normalizeVariables(rep.variables);
    }
  } catch {
    if (requestId === variablesRequestSeq) {
      effectiveVariables.value = [];
    }
  }
}

watch(
  () => form.value.variables,
  () => {
    void refreshEffectiveVariables();
  },
  { deep: true }
);

onMounted(() => {
  void load();
});
</script>

<template>
  <n-alert v-if="errorMessage" class="form-alert" type="error" :show-icon="false">{{ errorMessage }}</n-alert>
  <p v-if="loading" class="muted">Loading...</p>

  <form v-else class="stack" @submit.prevent="submit">
    <label class="field">
      <span>Base URL</span>
      <n-input v-model:value="form.base_url" placeholder="https://public.example.com" required />
    </label>

    <h3 class="section-title">CORS</h3>
    <n-space>
      <n-switch v-model:value="form.cors.enabled">
        <template #checked>Enabled</template>
        <template #unchecked>Disabled</template>
      </n-switch>
      <n-checkbox v-model:checked="form.cors.allow_credentials">Allow Credentials</n-checkbox>
    </n-space>
    <label class="field">
      <span>Max Age</span>
      <n-input v-model:value="form.cors.max_age" />
    </label>
    <label class="field">
      <span>Allow Origins (one per line)</span>
      <n-input v-model:value="allowOriginsText" type="textarea" :autosize="{ minRows: 4 }" />
    </label>
    <label class="field">
      <span>Allow Methods (one per line)</span>
      <n-input v-model:value="allowMethodsText" type="textarea" :autosize="{ minRows: 4 }" />
    </label>
    <label class="field">
      <span>Allow Headers (one per line)</span>
      <n-input v-model:value="allowHeadersText" type="textarea" :autosize="{ minRows: 4 }" />
    </label>

    <h3 class="section-title">JWT Providers</h3>
    <label class="field">
      <span>JWK URLs (one per line)</span>
      <n-input v-model:value="jwkUrlsText" type="textarea" :autosize="{ minRows: 5 }" placeholder="https://example.com/.well-known/jwks.json" />
    </label>

    <h3 class="section-title">Variables</h3>
    <div class="field">
      <span>Variables</span>
      <VariableEditor v-model="form.variables" :available-variables="effectiveVariables" />
    </div>

    <h3 class="section-title">Auth</h3>
    <div class="field">
      <span>Auth</span>
      <AuthEditor v-if="canEdit" v-model="form.auth" :jwt-kid-options="jwtKidOptions" :variable-options="effectiveVariables" />
      <AuthCard v-else :auth="form.auth" title="" />
    </div>

    <div v-if="canEdit" class="actions">
      <n-button type="primary" attr-type="submit" :loading="saving">
        {{ saving ? "Saving..." : "Save Root" }}
      </n-button>
    </div>
  </form>
</template>
