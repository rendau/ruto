<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import {
  NButton,
  NCheckbox,
  NCollapse,
  NCollapseItem,
  NDrawer,
  NDrawerContent,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NInputGroup,
  useMessage,
  type FormItemRule
} from "naive-ui";
import { SearchOutline } from "@vicons/ionicons5";
import { createApp, getAppSwaggerUrlByBackendUrl, updateApp } from "@/api/app";
import { apiErrorMessage } from "@/api/http";
import { emptyApp } from "@/lib/entities";
import { useAppForm } from "@/composables/useAppForm";
import { useEntityForm } from "@/composables/useEntityForm";
import { useIsMobile } from "@/composables/useIsMobile";
import { useAppsStore } from "@/stores/apps";
import { useRootStore } from "@/stores/root";
import AuthEditor from "@/components/editors/AuthEditor.vue";
import LoggingEditor from "@/components/editors/LoggingEditor.vue";
import VariableEditor from "@/components/editors/VariableEditor.vue";
import KeyValueTextarea from "@/components/editors/KeyValueTextarea.vue";
import SwitchField from "@/components/common/SwitchField.vue";
import type { AppCreateRep, AppMain } from "@/api/types";

const router = useRouter();
const message = useMessage();
const appForm = useAppForm();
const appsStore = useAppsStore();
const rootStore = useRootStore();
const isMobile = useIsMobile();

const model = reactive<AppMain>(emptyApp());
const detecting = ref(false);
const expandedNames = ref<string[]>([]);

function nonDefaultSections(): string[] {
  const names: string[] = [];
  if (
    Object.keys(model.backend.headers).length ||
    Object.keys(model.backend.query_params).length
  ) {
    names.push("backend-extra");
  }
  // Default auth = enabled + extend with no methods; expand when it deviates
  // (disabled, replace mode, or any methods configured).
  if (!model.auth.enabled || model.auth.mode === "replace" || model.auth.methods.length) {
    names.push("auth");
  }
  const lg = model.logging;
  if (lg.level !== "" || lg.mode !== "extend" || lg.headers || lg.query_params || lg.req_body || lg.resp_body) {
    names.push("logging");
  }
  if (model.variables.length) {
    names.push("variables");
  }
  return names;
}

const inheritedVariables = computed(() => rootStore.root?.variables ?? []);
const authVariables = computed(() => [...inheritedVariables.value, ...model.variables]);

const idRule: FormItemRule = {
  required: true,
  message: "Application id is required",
  trigger: ["blur", "input"]
};
const backendRule: FormItemRule = {
  required: true,
  message: "Backend URL is required",
  trigger: ["blur", "input"]
};

const { formRef, submitting, isEdit, submit } = useEntityForm<AppMain, AppCreateRep>({
  show: () => appForm.show.value,
  entity: () => appForm.app.value,
  seed: (app) => {
    Object.assign(model, app ? (JSON.parse(JSON.stringify(app)) as AppMain) : emptyApp());
    expandedNames.value = nonDefaultSections();
    void rootStore.ensureLoaded();
  },
  create: () => createApp(model),
  update: () => updateApp(model),
  messages: { created: "Application created", updated: "Application saved" },
  onSaved: (created) => {
    void appsStore.refresh();
    window.dispatchEvent(new CustomEvent("app:saved", { detail: { id: created?.id || model.id } }));
    appForm.close();
    if (created?.id) {
      void router.push({ name: "app-workspace", params: { id: created.id } });
    }
  }
});

async function detectSwagger(): Promise<void> {
  if (!model.backend.url.trim()) return;
  detecting.value = true;
  try {
    const rep = await getAppSwaggerUrlByBackendUrl(model.backend.url.trim());
    if (rep.swagger_url) {
      model.backend.swagger_url = rep.swagger_url;
      message.success("Swagger URL detected");
    } else {
      message.info("No swagger URL found for this backend");
    }
  } catch (error) {
    message.error(apiErrorMessage(error, "Could not detect swagger URL"));
  } finally {
    detecting.value = false;
  }
}
</script>

<template>
  <NDrawer
    :show="appForm.show.value"
    :width="isMobile ? '100%' : 680"
    placement="right"
    :auto-focus="false"
    @update:show="(value: boolean) => { if (!value) appForm.close(); }"
  >
    <NDrawerContent :title="isEdit ? 'Edit application' : 'New application'" closable>
      <NForm ref="formRef" :model="model" :disabled="submitting" label-placement="top">
        <div class="form-grid">
          <NFormItem label="Application id" path="id" :rule="idRule">
            <NInput
              v-model:value="model.id"
              :disabled="isEdit"
              placeholder="users-service"
            />
          </NFormItem>
          <NFormItem label="Display name" path="name">
            <NInput v-model:value="model.name" placeholder="Users service" />
          </NFormItem>
        </div>

        <NFormItem label="Path prefix" path="path_prefix">
          <NInput v-model:value="model.path_prefix" placeholder="/users">
            <template #prefix><span class="muted mono">prefix</span></template>
          </NInput>
        </NFormItem>

        <div class="switch-row">
          <SwitchField v-model="model.active" label="Active" />
          <NCheckbox v-model:checked="model.exclude_from_metrics">Exclude from metrics</NCheckbox>
        </div>

        <h4 class="form-section">Backend</h4>
        <NFormItem label="Backend URL" path="backend.url" :rule="backendRule">
          <NInputGroup>
            <NInput v-model:value="model.backend.url" placeholder="http://users:8080" />
            <NButton :loading="detecting" tertiary @click="detectSwagger">
              <template #icon><NIcon :component="SearchOutline" /></template>
              Detect swagger
            </NButton>
          </NInputGroup>
        </NFormItem>
        <div class="form-grid">
          <NFormItem label="Swagger URL" path="backend.swagger_url">
            <NInput v-model:value="model.backend.swagger_url" placeholder="http://users:8080/swagger.json" />
          </NFormItem>
          <NFormItem label="gRPC URL" path="backend.grpc_url">
            <NInput v-model:value="model.backend.grpc_url" placeholder="users:9090" />
          </NFormItem>
        </div>

        <NCollapse v-model:expanded-names="expandedNames" class="advanced">
          <NCollapseItem title="Backend headers & query params" name="backend-extra">
            <div class="stacked">
              <label class="field">
                <span class="field__label">Headers (one per line, "Name: value")</span>
                <KeyValueTextarea
                  v-model="model.backend.headers"
                  :variables="authVariables"
                  placeholder="X-Internal: 1"
                />
              </label>
              <label class="field">
                <span class="field__label">Query params (one per line, "name: value")</span>
                <KeyValueTextarea
                  v-model="model.backend.query_params"
                  :variables="authVariables"
                  placeholder="env: prod"
                />
              </label>
            </div>
          </NCollapseItem>
          <NCollapseItem title="Authentication" name="auth">
            <AuthEditor
              v-model="model.auth"
              :jwt-kid-options="rootStore.jwtKids"
              :variable-options="authVariables"
            />
          </NCollapseItem>
          <NCollapseItem title="Logging" name="logging">
            <LoggingEditor v-model="model.logging" />
          </NCollapseItem>
          <NCollapseItem title="Variables" name="variables">
            <p class="muted form-hint">
              App variables override Root variables and are referenced as
              <code v-pre>{{name}}</code> in this app's config.
            </p>
            <VariableEditor v-model="model.variables" :available-variables="inheritedVariables" />
          </NCollapseItem>
        </NCollapse>
      </NForm>

      <template #footer>
        <div class="form-actions">
          <NButton :disabled="submitting" @click="appForm.close()">Cancel</NButton>
          <NButton type="primary" :loading="submitting" @click="submit">
            {{ isEdit ? "Save changes" : "Create application" }}
          </NButton>
        </div>
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.switch-row {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 18px;
}

.form-section {
  margin: 6px 0 12px;
  font-size: 13px;
  font-weight: 600;
  color: var(--c-text-2);
  padding-bottom: 8px;
  border-bottom: 1px solid var(--c-border);
}

.advanced {
  margin-top: 10px;
}

.stacked {
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

.form-hint {
  margin: 0 0 10px;
  font-size: 12.5px;
}

.form-hint code {
  font-family: var(--font-mono);
  color: var(--c-text-2);
}

@media (max-width: 560px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
