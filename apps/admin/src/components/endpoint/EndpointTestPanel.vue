<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { NAlert, NButton, NCollapse, NCollapseItem, NInput, NModal, NTag } from "naive-ui";
import { testEndpointRequest } from "@/api/endpoint";
import { apiErrorMessage } from "@/api/http";
import { joinUrl, maybePrettyJson } from "@/lib/format";
import VariableEditor from "@/components/editors/VariableEditor.vue";
import MethodBadge from "@/components/common/MethodBadge.vue";
import JsonBlock from "@/components/common/JsonBlock.vue";
import type { AppMain, EndpointMain, EndpointTestResponse, Variable } from "@/api/types";

const props = defineProps<{
  show: boolean;
  endpoint: EndpointMain | null;
  app: AppMain | null;
}>();

const emit = defineEmits<{ "update:show": [value: boolean] }>();

const PATH_PARAM_PATTERN = /\{([^/{}]+)\}/g;
const METHODS_WITH_BODY = new Set(["POST", "PUT", "PATCH", "DELETE"]);

const pathParams = ref<Variable[]>([]);
const queryParams = ref<Variable[]>([]);
const body = ref("");
const loading = ref(false);
const errorMessage = ref("");
const response = ref<EndpointTestResponse | null>(null);

const method = computed(() => (props.endpoint?.http.method || "").trim().toUpperCase() || "GET");
const showBody = computed(() => METHODS_WITH_BODY.has(method.value));

const pathTemplate = computed(() => {
  const custom = (props.endpoint?.backend.custom_path || "").trim();
  return custom || (props.endpoint?.http.path || "").trim();
});

const pathTokens = computed(() => {
  const names: string[] = [];
  const seen = new Set<string>();
  for (const match of pathTemplate.value.matchAll(PATH_PARAM_PATTERN)) {
    const name = match[1];
    if (name && !seen.has(name)) {
      seen.add(name);
      names.push(name);
    }
  }
  return names;
});

const pathParamMap = computed(() => {
  const map = new Map<string, string>();
  for (const item of pathParams.value) {
    map.set(item.key, item.value);
  }
  return map;
});

const resolvedPath = computed(() =>
  pathTemplate.value.replace(PATH_PARAM_PATTERN, (token, name: string) => {
    const value = pathParamMap.value.get(name);
    return value ? encodeURIComponent(value) : token;
  })
);

const previewUrl = computed(() =>
  props.app ? joinUrl(props.app.backend.url, resolvedPath.value) : resolvedPath.value
);

const statusTagType = computed<"success" | "warning" | "error" | "default">(() => {
  if (!response.value) return "default";
  if (response.value.error) return "error";
  const code = response.value.status_code;
  if (code >= 200 && code < 300) return "success";
  if (code >= 300 && code < 500) return "warning";
  return "error";
});

const prettyBody = computed(() => maybePrettyJson(response.value?.body || ""));

function resetForm(): void {
  pathParams.value = pathTokens.value.map((name) => ({ key: name, value: "" }));
  queryParams.value = [];
  body.value = "";
  response.value = null;
  errorMessage.value = "";
}

async function send(): Promise<void> {
  if (!props.endpoint) return;
  loading.value = true;
  errorMessage.value = "";
  try {
    response.value = await testEndpointRequest(props.endpoint.id, {
      path_params: pathParams.value,
      query_params: queryParams.value,
      body: body.value
    });
  } catch (error) {
    errorMessage.value = apiErrorMessage(error, "Test request failed");
  } finally {
    loading.value = false;
  }
}

watch(
  () => props.show,
  (show) => {
    if (show) resetForm();
  }
);
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="Test request"
    class="test-modal"
    :bordered="false"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <div v-if="endpoint" class="test">
      <div class="test__line">
        <MethodBadge :method="method" />
        <code class="test__url">{{ previewUrl }}</code>
      </div>

      <div v-if="pathTokens.length" class="test__field">
        <span class="section-label">Path variables</span>
        <div class="test__path-grid">
          <div v-for="item in pathParams" :key="item.key" class="test__path-row">
            <code class="test__path-name">{{ item.key }}</code>
            <NInput
              :value="item.value"
              :placeholder="`value for {${item.key}}`"
              @update:value="(value: string) => (item.value = value)"
            />
          </div>
        </div>
      </div>

      <div class="test__field">
        <span class="section-label">Query parameters</span>
        <VariableEditor v-model="queryParams" add-label="Add query param" />
        <span class="muted test__hint">
          The endpoint's configured headers and query params are applied automatically on the server.
        </span>
      </div>

      <div v-if="showBody" class="test__field">
        <span class="section-label">Body</span>
        <NInput
          v-model:value="body"
          type="textarea"
          placeholder="Request body (JSON or raw text)"
          :autosize="{ minRows: 4, maxRows: 12 }"
        />
      </div>

      <div class="test__actions">
        <NButton type="primary" :loading="loading" @click="send">Send request</NButton>
      </div>

      <NAlert v-if="errorMessage" type="error" :bordered="false">{{ errorMessage }}</NAlert>

      <section v-if="response" class="test__response">
        <div class="test__response-head">
          <NTag :type="statusTagType" size="medium" :bordered="false">
            {{ response.error ? "Failed" : response.status_code }}
          </NTag>
          <span class="test__duration mono">{{ response.duration_ms }} ms</span>
          <code class="test__response-url">{{ response.request_method }} {{ response.request_url }}</code>
        </div>

        <NAlert v-if="response.error" type="error" :bordered="false">{{ response.error }}</NAlert>

        <template v-else>
          <NCollapse>
            <NCollapseItem :title="`Headers (${response.headers.length})`" name="headers">
              <div v-if="response.headers.length" class="test__headers">
                <div v-for="(header, index) in response.headers" :key="index" class="test__header-row">
                  <code class="test__header-key">{{ header.key }}</code>
                  <code class="test__header-value">{{ header.value }}</code>
                </div>
              </div>
              <p v-else class="muted">No headers.</p>
            </NCollapseItem>
          </NCollapse>

          <div class="test__field">
            <span class="section-label">Body</span>
            <JsonBlock v-if="prettyBody" :content="prettyBody" />
            <p v-else class="muted">Empty response body.</p>
          </div>
        </template>
      </section>
    </div>
  </NModal>
</template>

<style scoped>
:global(.test-modal) {
  width: min(820px, calc(100vw - 48px));
}

.test {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.test__line {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 10px 12px;
  border: 1px solid var(--c-border);
  border-radius: 9px;
  background: var(--c-surface-2);
}

.test__url {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--c-text);
  overflow-wrap: anywhere;
}

.test__field {
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.test__hint {
  font-size: 12px;
}

.test__path-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.test__path-row {
  display: grid;
  grid-template-columns: minmax(120px, 200px) 1fr;
  gap: 8px;
  align-items: center;
}

.test__path-name {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--c-teal);
}

.test__actions {
  display: flex;
  justify-content: flex-end;
}

.test__response {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-top: 16px;
  border-top: 1px solid var(--c-border);
}

.test__response-head {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.test__duration {
  font-size: 13px;
  font-weight: 700;
  color: var(--c-teal);
}

.test__response-url {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--c-text-3);
  overflow-wrap: anywhere;
}

.test__headers {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.test__header-row {
  display: grid;
  grid-template-columns: minmax(140px, 240px) 1fr;
  gap: 10px;
}

.test__header-key {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--c-teal);
  overflow-wrap: anywhere;
}

.test__header-value {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--c-text);
  overflow-wrap: anywhere;
}

@media (max-width: 640px) {
  .test__path-row,
  .test__header-row {
    grid-template-columns: 1fr;
  }
}
</style>
