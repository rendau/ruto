<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import {
  NAlert,
  NButton,
  NCheckbox,
  NDynamicInput,
  NIcon,
  NInput,
  NSpin,
  NTag,
  useMessage
} from "naive-ui";
import { SaveOutline } from "@vicons/ionicons5";
import { getRoot, setRoot } from "@/api/root";
import { emptyAuth, emptyLogging } from "@/api/normalize";
import { apiErrorMessage } from "@/api/http";
import { useRootStore } from "@/stores/root";
import { useAuthStore } from "@/stores/auth";
import { arrayToLines, linesToArray } from "@/lib/forms";
import PageContainer from "@/components/common/PageContainer.vue";
import SectionCard from "@/components/common/SectionCard.vue";
import SwitchField from "@/components/common/SwitchField.vue";
import AuthEditor from "@/components/editors/AuthEditor.vue";
import LoggingEditor from "@/components/editors/LoggingEditor.vue";
import VariableEditor from "@/components/editors/VariableEditor.vue";
import type { RootMain } from "@/api/types";

const message = useMessage();
const rootStore = useRootStore();
const authStore = useAuthStore();
const canEdit = computed(() => authStore.isAdmin);

const loading = ref(false);
const saving = ref(false);

function emptyRoot(): RootMain {
  return {
    base_url: "",
    cors: {
      enabled: false,
      allow_credentials: false,
      max_age: "",
      allow_origins: [],
      allow_methods: [],
      allow_headers: []
    },
    jwt: [],
    auth: emptyAuth(),
    logging: emptyLogging(),
    log_own_response_errors: false,
    variables: []
  };
}

const model = reactive<RootMain>(emptyRoot());
const jwtUrls = ref<string[]>([]);
const corsText = reactive({ origins: "", methods: "", headers: "" });

const authVariables = computed(() => model.variables);

async function load(): Promise<void> {
  loading.value = true;
  try {
    const root = await getRoot();
    Object.assign(model, JSON.parse(JSON.stringify(root)) as RootMain);
    jwtUrls.value = model.jwt.map((item) => item.jwk_url);
    corsText.origins = arrayToLines(model.cors.allow_origins);
    corsText.methods = arrayToLines(model.cors.allow_methods);
    corsText.headers = arrayToLines(model.cors.allow_headers);
    rootStore.setRoot(root);
    void rootStore.loadJwtKids(jwtUrls.value);
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to load root configuration"));
  } finally {
    loading.value = false;
  }
}

let kidTimer: ReturnType<typeof setTimeout> | undefined;
watch(
  jwtUrls,
  (urls) => {
    clearTimeout(kidTimer);
    kidTimer = setTimeout(() => void rootStore.loadJwtKids(urls), 400);
  },
  { deep: true }
);

async function save(): Promise<void> {
  if (!canEdit.value) return;
  saving.value = true;
  try {
    const payload: RootMain = {
      ...model,
      jwt: jwtUrls.value.map((url) => ({ jwk_url: url.trim() })).filter((item) => item.jwk_url),
      cors: {
        ...model.cors,
        allow_origins: linesToArray(corsText.origins),
        allow_methods: linesToArray(corsText.methods),
        allow_headers: linesToArray(corsText.headers)
      }
    };
    await setRoot(payload);
    rootStore.setRoot(payload);
    void rootStore.loadJwtKids(jwtUrls.value);
    message.success("Root configuration saved");
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to save configuration"));
  } finally {
    saving.value = false;
  }
}

onMounted(load);
</script>

<template>
  <PageContainer :width="980">
    <div class="page-head">
      <div>
        <h1 class="page-head__title">Root configuration</h1>
        <p class="page-head__sub muted">Global gateway defaults inherited by every application</p>
      </div>
      <NButton v-if="canEdit" type="primary" :loading="saving" @click="save">
        <template #icon><NIcon :component="SaveOutline" /></template>
        Save
      </NButton>
    </div>

    <NAlert v-if="!canEdit" type="info" :bordered="false" class="ro-banner">
      Read-only — administrator rights are required to edit the root configuration.
    </NAlert>

    <NSpin :show="loading">
      <div class="sections" :class="{ 'is-locked': !canEdit }">
        <SectionCard title="General">
          <label class="field">
            <span class="field__label">Base URL</span>
            <NInput v-model:value="model.base_url" placeholder="https://api.example.com" />
          </label>
          <NCheckbox v-model:checked="model.log_own_response_errors" class="general__check">
            Log gateway's own response errors
          </NCheckbox>
        </SectionCard>

        <SectionCard title="CORS">
          <div class="cors__switches">
            <SwitchField v-model="model.cors.enabled" label="CORS enabled" />
            <NCheckbox v-model:checked="model.cors.allow_credentials">Allow credentials</NCheckbox>
          </div>
          <template v-if="model.cors.enabled">
            <label class="field">
              <span class="field__label">Max age</span>
              <NInput v-model:value="model.cors.max_age" placeholder="e.g. 86400 or 24h" />
            </label>
            <div class="cors__grid">
              <label class="field">
                <span class="field__label">Allow origins (one per line, * for any)</span>
                <NInput
                  v-model:value="corsText.origins"
                  type="textarea"
                  :autosize="{ minRows: 3, maxRows: 8 }"
                  placeholder="https://app.example.com"
                />
              </label>
              <label class="field">
                <span class="field__label">Allow methods (one per line)</span>
                <NInput
                  v-model:value="corsText.methods"
                  type="textarea"
                  :autosize="{ minRows: 3, maxRows: 8 }"
                  placeholder="GET"
                />
              </label>
              <label class="field">
                <span class="field__label">Allow headers (one per line)</span>
                <NInput
                  v-model:value="corsText.headers"
                  type="textarea"
                  :autosize="{ minRows: 3, maxRows: 8 }"
                  placeholder="Authorization"
                />
              </label>
            </div>
          </template>
        </SectionCard>

        <SectionCard
          title="JWT providers"
          description="JWKS endpoints whose key IDs become available to endpoint JWT auth"
        >
          <NDynamicInput
            v-model:value="jwtUrls"
            :min="0"
            placeholder="https://issuer/.well-known/jwks.json"
          />
          <div v-if="rootStore.jwtKids.length" class="kids">
            <span class="muted kids__label">Available KIDs:</span>
            <NTag
              v-for="kid in rootStore.jwtKids"
              :key="kid"
              size="small"
              :bordered="false"
              type="info"
            >
              {{ kid }}
            </NTag>
          </div>
        </SectionCard>

        <SectionCard title="Authentication">
          <AuthEditor
            v-model="model.auth"
            :jwt-kid-options="rootStore.jwtKids"
            :variable-options="authVariables"
            hide-mode
          />
        </SectionCard>

        <SectionCard title="Logging">
          <LoggingEditor v-model="model.logging" hide-mode />
        </SectionCard>

        <SectionCard
          title="Variables"
          description="Global variables inherited by every application"
        >
          <VariableEditor v-model="model.variables" />
        </SectionCard>
      </div>
    </NSpin>
  </PageContainer>
</template>

<style scoped>
.page-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.page-head__title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
}

.page-head__sub {
  margin: 3px 0 0;
  font-size: 13px;
}

.sections {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field__label {
  font-size: 12px;
  color: var(--c-text-3);
}

.general__check {
  margin-top: 14px;
}

.cors__switches {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 16px;
}

.cors__grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 14px;
  margin-top: 14px;
}

.kids {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 14px;
}

.kids__label {
  font-size: 12px;
}

@media (max-width: 760px) {
  .cors__grid {
    grid-template-columns: 1fr;
  }
}
</style>
