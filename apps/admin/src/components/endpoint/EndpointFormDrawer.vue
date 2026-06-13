<script setup lang="ts">
import { computed, reactive } from "vue";
import {
  NButton,
  NCheckbox,
  NCollapse,
  NCollapseItem,
  NDrawer,
  NDrawerContent,
  NForm,
  NFormItem,
  NInput,
  NRadioButton,
  NRadioGroup,
  NSelect,
  NSwitch,
  type FormItemRule
} from "naive-ui";
import { createEndpoint, updateEndpoint } from "@/api/endpoint";
import { emptyAuth, emptyLogging } from "@/api/normalize";
import { useEntityForm } from "@/composables/useEntityForm";
import { useRootStore } from "@/stores/root";
import { HTTP_METHOD_OPTIONS } from "@/constants/enums";
import AuthEditor from "@/components/editors/AuthEditor.vue";
import LoggingEditor from "@/components/editors/LoggingEditor.vue";
import VariableEditor from "@/components/editors/VariableEditor.vue";
import KeyValueTextarea from "@/components/editors/KeyValueTextarea.vue";
import type { AppMain, EndpointCreateRep, EndpointMain, EndpointType } from "@/api/types";

const props = defineProps<{
  show: boolean;
  endpoint: EndpointMain | null;
  app: AppMain | null;
  prefill?: Partial<EndpointMain> | null;
}>();

const emit = defineEmits<{ "update:show": [value: boolean]; saved: [] }>();

const rootStore = useRootStore();

function emptyEndpoint(appId: string): EndpointMain {
  return {
    id: "",
    app_id: appId,
    active: true,
    exclude_from_metrics: false,
    type: "http",
    http: { method: "GET", path: "" },
    grpc: { service: "", method: "", path: "" },
    backend: { custom_path: "", headers: {}, query_params: {} },
    auth: emptyAuth(),
    logging: emptyLogging(),
    variables: []
  };
}

const model = reactive<EndpointMain>(emptyEndpoint(""));

const grpcAvailable = computed(
  () => Boolean(props.app?.backend?.grpc_url?.trim()) || model.type === "grpc"
);

const inheritedVariables = computed(() => [
  ...(rootStore.root?.variables ?? []),
  ...(props.app?.variables ?? [])
]);
const authVariables = computed(() => [...inheritedVariables.value, ...model.variables]);

const httpPathRule = computed<FormItemRule>(() => ({
  required: model.type === "http",
  message: "Path is required",
  trigger: ["blur", "input"]
}));
const grpcServiceRule = computed<FormItemRule>(() => ({
  required: model.type === "grpc",
  message: "Service is required",
  trigger: ["blur", "input"]
}));
const grpcMethodRule = computed<FormItemRule>(() => ({
  required: model.type === "grpc",
  message: "Method is required",
  trigger: ["blur", "input"]
}));

const { formRef, submitting, isEdit, submit } = useEntityForm<EndpointMain, EndpointCreateRep>({
  show: () => props.show,
  entity: () => props.endpoint,
  seed: (endpoint) => {
    if (endpoint) {
      Object.assign(model, JSON.parse(JSON.stringify(endpoint)) as EndpointMain);
    } else {
      Object.assign(model, emptyEndpoint(props.app?.id || ""));
      applyPrefill();
    }
    void rootStore.ensureLoaded();
  },
  create: () => createEndpoint(model),
  update: () => updateEndpoint(model),
  messages: { created: "Endpoint created", updated: "Endpoint saved" },
  onSaved: () => {
    emit("saved");
    close();
  }
});

function applyPrefill(): void {
  const prefill = props.prefill;
  if (!prefill) return;
  if (prefill.type) model.type = prefill.type;
  if (prefill.http) Object.assign(model.http, prefill.http);
  if (prefill.grpc) Object.assign(model.grpc, prefill.grpc);
  if (prefill.id) model.id = prefill.id;
  if (prefill.type === "grpc") syncGrpcPath();
}

function setType(type: EndpointType): void {
  model.type = type;
  if (type === "grpc") syncGrpcPath();
}

function syncGrpcPath(): void {
  const service = model.grpc.service.trim();
  const method = model.grpc.method.trim();
  model.grpc.path = service && method ? `/${service}/${method}` : "";
}

function close(): void {
  emit("update:show", false);
}
</script>

<template>
  <NDrawer
    :show="show"
    :width="680"
    placement="right"
    :auto-focus="false"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <NDrawerContent :title="isEdit ? 'Edit endpoint' : 'New endpoint'" closable>
      <NForm ref="formRef" :model="model" :disabled="submitting" label-placement="top">
        <NFormItem label="Type">
          <NRadioGroup
            :value="model.type"
            :disabled="isEdit"
            @update:value="(value: EndpointType) => setType(value)"
          >
            <NRadioButton value="http">HTTP</NRadioButton>
            <NRadioButton value="grpc" :disabled="!grpcAvailable">gRPC</NRadioButton>
          </NRadioGroup>
        </NFormItem>

        <template v-if="model.type === 'http'">
          <div class="http-grid">
            <NFormItem label="Method" path="http.method">
              <NSelect v-model:value="model.http.method" :options="HTTP_METHOD_OPTIONS" />
            </NFormItem>
            <NFormItem label="Path" path="http.path" :rule="httpPathRule">
              <NInput v-model:value="model.http.path" placeholder="/users/{id}" />
            </NFormItem>
          </div>
        </template>

        <template v-else>
          <div class="form-grid">
            <NFormItem label="Service" path="grpc.service" :rule="grpcServiceRule">
              <NInput
                v-model:value="model.grpc.service"
                placeholder="package.UserService"
                @update:value="syncGrpcPath"
              />
            </NFormItem>
            <NFormItem label="Method" path="grpc.method" :rule="grpcMethodRule">
              <NInput
                v-model:value="model.grpc.method"
                placeholder="GetUser"
                @update:value="syncGrpcPath"
              />
            </NFormItem>
          </div>
          <NFormItem label="Full path" path="grpc.path">
            <NInput v-model:value="model.grpc.path" placeholder="/package.UserService/GetUser" />
          </NFormItem>
        </template>

        <NFormItem v-if="!isEdit" label="Endpoint id (optional)">
          <NInput v-model:value="model.id" placeholder="Leave empty to auto-generate" />
        </NFormItem>

        <div class="switch-row">
          <NSwitch v-model:value="model.active">
            <template #checked>Active</template>
            <template #unchecked>Inactive</template>
          </NSwitch>
          <NCheckbox v-model:checked="model.exclude_from_metrics">Exclude from metrics</NCheckbox>
        </div>

        <NCollapse class="advanced">
          <NCollapseItem title="Backend overrides" name="backend">
            <div class="stacked">
              <label class="field">
                <span class="field__label">Custom backend path (optional)</span>
                <NInput v-model:value="model.backend.custom_path" placeholder="/internal/users/{id}" />
              </label>
              <label class="field">
                <span class="field__label">Headers (one per line, "Name: value")</span>
                <KeyValueTextarea v-model="model.backend.headers" placeholder="X-Scope: read" />
              </label>
              <label class="field">
                <span class="field__label">Query params (one per line, "name: value")</span>
                <KeyValueTextarea v-model="model.backend.query_params" placeholder="expand: true" />
              </label>
            </div>
          </NCollapseItem>
          <NCollapseItem title="Authentication" name="auth">
            <p class="muted form-hint">Inherited from the application unless overridden here.</p>
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
            <VariableEditor v-model="model.variables" :available-variables="inheritedVariables" />
          </NCollapseItem>
        </NCollapse>
      </NForm>

      <template #footer>
        <NButton :disabled="submitting" @click="close">Cancel</NButton>
        <NButton type="primary" :loading="submitting" @click="submit">
          {{ isEdit ? "Save changes" : "Create endpoint" }}
        </NButton>
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.http-grid {
  display: grid;
  grid-template-columns: 160px 1fr;
  gap: 14px;
}

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

.advanced {
  margin-top: 6px;
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

@media (max-width: 560px) {
  .http-grid,
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
