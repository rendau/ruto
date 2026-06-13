<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import VariableEditor from "./VariableEditor.vue";
import { testEndpointRequest } from "../lib/api";
import { notifyError } from "../lib/notify";
import type { AppMain, EndpointMain, EndpointTestResponse, Variable } from "../types/api";

const props = defineProps<{
  endpoint: EndpointMain;
  app: AppMain | null;
  open: boolean;
}>();

const emit = defineEmits<{
  (e: "close"): void;
}>();

const PATH_PARAM_PATTERN = /\{([^/{}]+)\}/g;
const METHODS_WITH_BODY = new Set(["POST", "PUT", "PATCH", "DELETE"]);

const pathParams = ref<Variable[]>([]);
const queryParams = ref<Variable[]>([]);
const body = ref("");

const loading = ref(false);
const errorMessage = ref("");
const response = ref<EndpointTestResponse | null>(null);

const method = computed(() => (props.endpoint.http.method || "").trim().toUpperCase() || "GET");
const showBody = computed(() => METHODS_WITH_BODY.has(method.value));

// The path template that the backend will use: custom_path if set, else
// http.path. The gateway strips app.path_prefix before proxying, so it is not
// part of the backend request. Path tokens use single-brace {name}.
const pathTemplate = computed(() => {
  const customPath = (props.endpoint.backend.custom_path || "").trim();
  return customPath || (props.endpoint.http.path || "").trim();
});

const pathTokens = computed(() => {
  const names: string[] = [];
  const seen = new Set<string>();
  for (const match of pathTemplate.value.matchAll(PATH_PARAM_PATTERN)) {
    const name = match[1];
    if (!seen.has(name)) {
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

// Live preview of the concrete path with filled {name} values substituted.
const resolvedPath = computed(() =>
  pathTemplate.value.replace(PATH_PARAM_PATTERN, (token, name: string) => {
    const value = pathParamMap.value.get(name);
    return value ? encodeURIComponent(value) : token;
  })
);

const previewUrl = computed(() => {
  if (!props.app) {
    return resolvedPath.value;
  }
  return joinUrl(props.app.backend.url, resolvedPath.value);
});

const statusTagType = computed<"success" | "warning" | "error" | "default">(() => {
  if (!response.value) {
    return "default";
  }
  if (response.value.error) {
    return "error";
  }
  const code = response.value.status_code;
  if (code >= 200 && code < 300) {
    return "success";
  }
  if (code >= 300 && code < 500) {
    return "warning";
  }
  return "error";
});

const prettyBody = computed(() => {
  const raw = response.value?.body || "";
  if (!raw) {
    return "";
  }
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
});

function joinUrl(baseUrl: string, path: string): string {
  const base = (baseUrl || "").trim().replace(/\/+$/g, "");
  if (!base) {
    return path;
  }
  if (!path) {
    return base;
  }
  return `${base}${path.startsWith("/") ? "" : "/"}${path}`;
}

function resetForm(): void {
  pathParams.value = pathTokens.value.map((name) => ({ key: name, value: "" }));
  queryParams.value = [];
  body.value = "";
  response.value = null;
  errorMessage.value = "";
}

async function send(): Promise<void> {
  loading.value = true;
  errorMessage.value = "";
  try {
    response.value = await testEndpointRequest(props.endpoint.id, {
      path_params: pathParams.value,
      query_params: queryParams.value,
      body: body.value
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    errorMessage.value = message;
    notifyError("Test request failed");
  } finally {
    loading.value = false;
  }
}

function close(): void {
  if (!props.open) {
    return;
  }
  emit("close");
}

function onKeydown(event: KeyboardEvent): void {
  if (props.open && event.key === "Escape") {
    close();
  }
}

watch(
  () => props.open,
  (open) => {
    if (open) {
      resetForm();
    }
  }
);

onMounted(() => {
  window.addEventListener("keydown", onKeydown);
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", onKeydown);
});
</script>

<template>
  <n-modal
    :show="open"
    preset="card"
    class="endpoint-test-card"
    title="Test request"
    :bordered="false"
    :mask-closable="true"
    content-style="display: flex; min-height: 0; height: 100%; overflow: hidden;"
    @update:show="(value: boolean) => { if (!value) close(); }"
  >
    <div class="endpoint-test-content">
    <div class="endpoint-test-body">
      <section class="test-section">
        <div class="test-request-line">
          <span class="http-method-badge" :class="`method-${method.toLowerCase()}`">{{ method }}</span>
          <code class="test-url-preview">{{ previewUrl }}</code>
        </div>

        <div v-if="pathTokens.length" class="test-field">
          <span class="test-label">Path variables</span>
          <div class="test-path-grid">
            <div v-for="item in pathParams" :key="item.key" class="test-path-row">
              <code class="test-path-name">{{ item.key }}</code>
              <n-input
                :value="item.value"
                :placeholder="`value for {${item.key}}`"
                @update:value="(value: string) => (item.value = value)"
              />
            </div>
          </div>
        </div>

        <div class="test-field">
          <span class="test-label">Query parameters</span>
          <VariableEditor v-model="queryParams" />
          <span class="test-hint">Endpoint's configured headers and query params are applied automatically on the server.</span>
        </div>

        <div v-if="showBody" class="test-field">
          <span class="test-label">Body</span>
          <n-input
            v-model:value="body"
            type="textarea"
            placeholder="Request body (JSON or raw text)"
            :autosize="{ minRows: 4, maxRows: 12 }"
          />
        </div>

        <div class="test-actions">
          <n-button type="primary" :loading="loading" @click="send">Send request</n-button>
        </div>
      </section>

      <n-alert v-if="errorMessage" type="error" :show-icon="false">{{ errorMessage }}</n-alert>

      <section v-if="response" class="test-section test-response">
        <div class="test-response-head">
          <n-tag :type="statusTagType" size="medium">
            {{ response.error ? "Failed" : response.status_code }}
          </n-tag>
          <span class="test-duration">{{ response.duration_ms }} ms</span>
          <code class="test-response-url">{{ response.request_method }} {{ response.request_url }}</code>
        </div>

        <n-alert v-if="response.error" type="error" :show-icon="false">{{ response.error }}</n-alert>

        <template v-else>
          <n-collapse>
            <n-collapse-item :title="`Headers (${response.headers.length})`" name="headers">
              <div v-if="response.headers.length" class="test-headers">
                <div v-for="(header, index) in response.headers" :key="index" class="test-header-row">
                  <code class="test-header-key">{{ header.key }}</code>
                  <code class="test-header-value">{{ header.value }}</code>
                </div>
              </div>
              <p v-else class="muted">No headers.</p>
            </n-collapse-item>
          </n-collapse>

          <div class="test-field">
            <span class="test-label">Body</span>
            <pre v-if="prettyBody" class="test-body-output">{{ prettyBody }}</pre>
            <p v-else class="muted">Empty response body.</p>
          </div>
        </template>
      </section>
    </div>
    </div>
  </n-modal>
</template>

<style scoped>
:global(.endpoint-test-card) {
  width: min(820px, calc(100vw - 48px));
  max-height: calc(100dvh - 48px);
  margin: auto;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

:global(.endpoint-test-card .n-card__content) {
  flex: 1 1 auto;
  min-height: 0;
  height: 100%;
  display: flex;
  overflow: hidden;
  max-height: none;
}

.endpoint-test-content {
  flex: 1 1 auto;
  min-height: 0;
  width: 100%;
  overflow-y: auto;
}

.endpoint-test-body {
  display: grid;
  gap: 16px;
  align-content: start;
}

.test-section {
  display: grid;
  gap: 14px;
}

.test-request-line {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 10px 12px;
  border: 1px solid #3f597c;
  border-radius: 8px;
  background: #1e2d44;
}

.test-url-preview {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 13px;
  color: #f4f8ff;
  overflow-wrap: anywhere;
}

.test-field {
  display: grid;
  gap: 6px;
}

.test-label {
  color: #9cb1cd;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}

.test-hint {
  color: #8ea3c0;
  font-size: 12px;
}

.test-path-grid {
  display: grid;
  gap: 8px;
}

.test-path-row {
  display: grid;
  grid-template-columns: minmax(120px, 200px) 1fr;
  gap: 8px;
  align-items: center;
}

.test-path-name {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 13px;
  color: #c8e7ff;
}

.test-actions {
  display: flex;
  justify-content: flex-end;
}

.test-response {
  border-top: 1px solid #2b3e5b;
  padding-top: 16px;
}

.test-response-head {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.test-duration {
  color: #c8e7ff;
  font-size: 13px;
  font-weight: 700;
}

.test-response-url {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  color: #9eb2cf;
  overflow-wrap: anywhere;
}

.test-headers {
  display: grid;
  gap: 6px;
}

.test-header-row {
  display: grid;
  grid-template-columns: minmax(140px, 240px) 1fr;
  gap: 10px;
}

.test-header-key {
  color: #c8e7ff;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  overflow-wrap: anywhere;
}

.test-header-value {
  color: #e6eefb;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  overflow-wrap: anywhere;
}

.test-body-output {
  margin: 0;
  padding: 12px 14px;
  border: 1px solid #334b70;
  border-radius: 8px;
  background: #101827;
  color: #d4e4fa;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 13px;
  line-height: 1.5;
  max-height: 360px;
  overflow: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

@media (max-width: 720px) {
  :global(.endpoint-test-card) {
    width: calc(100vw - 16px);
    max-height: calc(100dvh - 16px);
    margin: 8px auto;
  }

  .test-path-row,
  .test-header-row {
    grid-template-columns: 1fr;
  }
}
</style>
