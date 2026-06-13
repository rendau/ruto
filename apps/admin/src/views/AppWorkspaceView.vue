<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  NButton,
  NIcon,
  NInput,
  NSelect,
  NSpin,
  NSwitch,
  NTabPane,
  NTabs,
  NTag,
  useMessage
} from "naive-ui";
import {
  AddOutline,
  CreateOutline,
  FlashOutline,
  LockClosedOutline,
  RefreshOutline,
  SearchOutline,
  TerminalOutline,
  TrashOutline
} from "@vicons/ionicons5";
import { deleteApp, getApp, getAppInterpolate, updateApp } from "@/api/app";
import { listEndpoints } from "@/api/endpoint";
import { apiErrorMessage } from "@/api/http";
import { variablesToArray } from "@/api/normalize";
import { useAppsStore } from "@/stores/apps";
import { useRootStore } from "@/stores/root";
import { useAppForm } from "@/composables/useAppForm";
import { useConfirm } from "@/composables/useConfirm";
import { useAuthStore } from "@/stores/auth";
import { HTTP_METHOD_OPTIONS } from "@/constants/enums";
import { joinPath, joinUrl } from "@/lib/format";
import PageContainer from "@/components/common/PageContainer.vue";
import SectionCard from "@/components/common/SectionCard.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import StatusTag from "@/components/common/StatusTag.vue";
import MethodBadge from "@/components/common/MethodBadge.vue";
import CopyText from "@/components/common/CopyText.vue";
import KeyValueGrid from "@/components/common/KeyValueGrid.vue";
import AuthSummary from "@/components/display/AuthSummary.vue";
import LoggingSummary from "@/components/display/LoggingSummary.vue";
import EndpointFormDrawer from "@/components/endpoint/EndpointFormDrawer.vue";
import EndpointDetailDrawer from "@/components/endpoint/EndpointDetailDrawer.vue";
import EndpointTestPanel from "@/components/endpoint/EndpointTestPanel.vue";
import SwaggerSyncPanel from "@/components/endpoint/SwaggerSyncPanel.vue";
import GrpcReflectionPanel from "@/components/endpoint/GrpcReflectionPanel.vue";
import GrpcInstructionPanel from "@/components/endpoint/GrpcInstructionPanel.vue";
import type { AppMain, EndpointMain } from "@/api/types";

const route = useRoute();
const router = useRouter();
const message = useMessage();
const appsStore = useAppsStore();
const rootStore = useRootStore();
const authStore = useAuthStore();
const appForm = useAppForm();
const { confirmDelete } = useConfirm();

const appId = computed(() => (typeof route.params.id === "string" ? route.params.id : ""));
const canEdit = computed(() => authStore.isAdmin);

const app = ref<AppMain | null>(null);
const endpoints = ref<EndpointMain[]>([]);
const loadingApp = ref(false);
const loadingEndpoints = ref(false);
const togglingApp = ref(false);

const interpolatedApp = ref<AppMain | null>(null);
const showInterpolatedVars = ref(false);

const protocol = ref<"http" | "grpc">("http");
const filters = reactive({ search: "", method: null as string | null, status: "all", auth: "all" });

// ---- Endpoint drawers/panels ---------------------------------------------

const showForm = ref(false);
const editingEndpoint = ref<EndpointMain | null>(null);
const formPrefill = ref<Partial<EndpointMain> | null>(null);
const showDetail = ref(false);
const detailId = ref<string | null>(null);
const showTest = ref(false);
const testEndpoint = ref<EndpointMain | null>(null);
const showSwagger = ref(false);
const showGrpcReflection = ref(false);
const showGrpcInstruction = ref(false);

const hasGrpc = computed(() => Boolean(app.value?.backend?.grpc_url?.trim()));
const showGrpcTab = computed(() => hasGrpc.value || endpoints.value.some((e) => e.type === "grpc"));

const publicBase = computed(() => {
  if (!app.value) return "";
  const path = joinPath(app.value.path_prefix);
  return rootStore.baseUrl ? joinUrl(rootStore.baseUrl, path) : path;
});

const previewVariables = computed(() => {
  const source = showInterpolatedVars.value ? interpolatedApp.value : app.value;
  return source?.variables ?? [];
});

const filteredEndpoints = computed(() => {
  const search = filters.search.trim().toLowerCase();
  return endpoints.value.filter((endpoint) => {
    if (endpoint.type !== protocol.value) return false;
    if (filters.status === "active" && !endpoint.active) return false;
    if (filters.status === "inactive" && endpoint.active) return false;
    if (filters.auth === "protected" && !endpoint.auth.enabled) return false;
    if (filters.auth === "public" && endpoint.auth.enabled) return false;
    if (protocol.value === "http" && filters.method && endpoint.http.method !== filters.method) {
      return false;
    }
    if (search) {
      const haystack = [
        endpoint.id,
        endpoint.http.path,
        endpoint.http.method,
        endpoint.grpc.service,
        endpoint.grpc.method,
        endpoint.grpc.path
      ]
        .join(" ")
        .toLowerCase();
      if (!haystack.includes(search)) return false;
    }
    return true;
  });
});

const statusOptions = [
  { label: "All", value: "all" },
  { label: "Active", value: "active" },
  { label: "Inactive", value: "inactive" }
];
const authOptions = [
  { label: "Any auth", value: "all" },
  { label: "Protected", value: "protected" },
  { label: "Public", value: "public" }
];

function toItems(record: Record<string, string>) {
  return variablesToArray(record);
}

async function loadApp(): Promise<void> {
  if (!appId.value) {
    app.value = null;
    return;
  }
  loadingApp.value = true;
  interpolatedApp.value = null;
  showInterpolatedVars.value = false;
  try {
    app.value = await getApp(appId.value);
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to load application"));
    app.value = null;
  } finally {
    loadingApp.value = false;
  }
}

async function loadEndpoints(): Promise<void> {
  if (!appId.value) {
    endpoints.value = [];
    return;
  }
  loadingEndpoints.value = true;
  try {
    const rep = await listEndpoints({ app_id: appId.value });
    endpoints.value = rep.results ?? [];
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to load endpoints"));
  } finally {
    loadingEndpoints.value = false;
  }
}

async function loadAll(): Promise<void> {
  await Promise.all([loadApp(), loadEndpoints()]);
  void rootStore.ensureLoaded();
}

watch(showInterpolatedVars, async (value) => {
  if (value && !interpolatedApp.value && app.value) {
    try {
      interpolatedApp.value = await getAppInterpolate({ id: app.value.id, variables: [] });
    } catch (error) {
      message.error(apiErrorMessage(error, "Failed to interpolate"));
      showInterpolatedVars.value = false;
    }
  }
});

// ---- App actions ----------------------------------------------------------

async function toggleApp(): Promise<void> {
  if (!app.value) return;
  togglingApp.value = true;
  try {
    await updateApp({ ...app.value, active: !app.value.active });
    message.success(app.value.active ? "Application deactivated" : "Application activated");
    await loadApp();
    void appsStore.refresh();
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to update application"));
  } finally {
    togglingApp.value = false;
  }
}

function removeApp(): void {
  const current = app.value;
  if (!current) return;
  confirmDelete({
    content: `Delete application "${current.name || current.id}" and all its endpoints? This cannot be undone.`,
    onConfirm: async () => {
      try {
        await deleteApp(current.id);
        message.success("Application deleted");
        await appsStore.refresh();
        void router.push({ name: "apps" });
      } catch (error) {
        message.error(apiErrorMessage(error, "Failed to delete application"));
      }
    }
  });
}

// ---- Endpoint actions -----------------------------------------------------

function openCreate(): void {
  editingEndpoint.value = null;
  formPrefill.value = protocol.value === "grpc" ? { type: "grpc" } : { type: "http" };
  showForm.value = true;
}

function openEdit(endpoint: EndpointMain): void {
  editingEndpoint.value = endpoint;
  formPrefill.value = null;
  showForm.value = true;
}

function openDetail(endpoint: EndpointMain): void {
  detailId.value = endpoint.id;
  showDetail.value = true;
}

function openTest(endpoint: EndpointMain): void {
  testEndpoint.value = endpoint;
  showTest.value = true;
}

function confirmDeleteEndpoint(endpoint: EndpointMain): void {
  confirmDelete({
    content: `Delete endpoint "${endpoint.id}"? This cannot be undone.`,
    onConfirm: async () => {
      try {
        const { deleteEndpoint } = await import("@/api/endpoint");
        await deleteEndpoint(endpoint.id);
        message.success("Endpoint deleted");
        await loadEndpoints();
      } catch (error) {
        message.error(apiErrorMessage(error, "Failed to delete endpoint"));
      }
    }
  });
}

function onDetailEdit(endpoint: EndpointMain): void {
  showDetail.value = false;
  openEdit(endpoint);
}

function onAppSaved(event: Event): void {
  const detail = (event as CustomEvent<{ id: string }>).detail;
  if (detail?.id === appId.value) {
    void loadApp();
  }
}

watch(
  () => route.params.id,
  () => void loadAll()
);

watch(showGrpcTab, (value) => {
  if (!value && protocol.value === "grpc") {
    protocol.value = "http";
  }
});

onMounted(() => {
  void loadAll();
  window.addEventListener("app:saved", onAppSaved);
});

onBeforeUnmount(() => {
  window.removeEventListener("app:saved", onAppSaved);
});
</script>

<template>
  <PageContainer :width="1280">
    <EmptyState
      v-if="!appId"
      description="Select an application from the sidebar to view its configuration and endpoints."
    >
      <NButton v-if="canEdit" type="primary" @click="appForm.open(null)">
        <template #icon><NIcon :component="AddOutline" /></template>
        New application
      </NButton>
    </EmptyState>

    <template v-else>
      <NSpin :show="loadingApp">
        <div v-if="app" class="workspace">
          <!-- Header -->
          <div class="app-head">
            <div class="app-head__main">
              <div class="app-head__title-row">
                <h1 class="app-head__title">{{ app.name || app.id }}</h1>
                <StatusTag :active="app.active" />
                <NTag v-if="hasGrpc" size="small" :bordered="false" class="grpc-tag">gRPC</NTag>
              </div>
              <div class="app-head__meta">
                <span class="mono app-head__id">{{ app.id }}</span>
                <span class="app-head__divider">·</span>
                <CopyText :value="publicBase" label="Public base URL" />
              </div>
            </div>
            <div v-if="canEdit" class="app-head__actions">
              <NSwitch :value="app.active" :loading="togglingApp" @update:value="toggleApp">
                <template #checked>Active</template>
                <template #unchecked>Inactive</template>
              </NSwitch>
              <NButton tertiary @click="appForm.open(app)">
                <template #icon><NIcon :component="CreateOutline" /></template>
                Edit
              </NButton>
              <NButton
                v-if="hasGrpc"
                tertiary
                @click="showGrpcInstruction = true"
              >
                <template #icon><NIcon :component="TerminalOutline" /></template>
                Connect
              </NButton>
              <NButton class="danger-icon-button" type="error" tertiary @click="removeApp">
                <template #icon><NIcon :component="TrashOutline" /></template>
              </NButton>
            </div>
          </div>

          <!-- Config summary -->
          <div class="config-grid">
            <SectionCard title="Backend">
              <div class="kv-list">
                <div class="kv-list__row">
                  <span class="kv-list__label">URL</span>
                  <CopyText :value="app.backend.url" label="Backend URL" />
                </div>
                <div v-if="app.backend.swagger_url" class="kv-list__row">
                  <span class="kv-list__label">Swagger</span>
                  <CopyText :value="app.backend.swagger_url" label="Swagger URL" />
                </div>
                <div v-if="app.backend.grpc_url" class="kv-list__row">
                  <span class="kv-list__label">gRPC</span>
                  <CopyText :value="app.backend.grpc_url" label="gRPC URL" />
                </div>
              </div>
              <div
                v-if="Object.keys(app.backend.headers).length || Object.keys(app.backend.query_params).length"
                class="backend-extras"
              >
                <div v-if="Object.keys(app.backend.headers).length">
                  <span class="muted backend-extras__label">Headers</span>
                  <KeyValueGrid :items="toItems(app.backend.headers)" />
                </div>
                <div v-if="Object.keys(app.backend.query_params).length">
                  <span class="muted backend-extras__label">Query params</span>
                  <KeyValueGrid :items="toItems(app.backend.query_params)" />
                </div>
              </div>
            </SectionCard>

            <SectionCard title="Authentication">
              <AuthSummary :auth="app.auth" />
            </SectionCard>

            <SectionCard title="Logging">
              <LoggingSummary :logging="app.logging" />
            </SectionCard>

            <SectionCard title="Variables">
              <template #extra>
                <NSwitch v-model:value="showInterpolatedVars" size="small">
                  <template #checked>resolved</template>
                  <template #unchecked>raw</template>
                </NSwitch>
              </template>
              <KeyValueGrid :items="previewVariables" empty-text="No variables" />
            </SectionCard>
          </div>

          <!-- Endpoints -->
          <SectionCard title="Endpoints" :description="`${endpoints.length} total`">
            <template #extra>
              <div class="ep-toolbar-actions">
                <NButton
                  v-if="canEdit && app.backend.swagger_url"
                  size="small"
                  tertiary
                  @click="showSwagger = true"
                >
                  Swagger sync
                </NButton>
                <NButton
                  v-if="canEdit && hasGrpc"
                  size="small"
                  tertiary
                  @click="showGrpcReflection = true"
                >
                  gRPC reflection
                </NButton>
                <NButton v-if="canEdit" size="small" type="primary" @click="openCreate">
                  <template #icon><NIcon :component="AddOutline" /></template>
                  New endpoint
                </NButton>
                <NButton size="small" quaternary circle :loading="loadingEndpoints" @click="loadEndpoints">
                  <template #icon><NIcon :component="RefreshOutline" /></template>
                </NButton>
              </div>
            </template>

            <NTabs
              v-if="showGrpcTab"
              v-model:value="protocol"
              type="line"
              size="small"
              class="protocol-tabs"
            >
              <NTabPane name="http" tab="HTTP" />
              <NTabPane name="grpc" tab="gRPC" />
            </NTabs>

            <div class="ep-filters">
              <NInput
                v-model:value="filters.search"
                size="small"
                placeholder="Search endpoints"
                clearable
                class="ep-filters__search"
              >
                <template #prefix><NIcon :component="SearchOutline" /></template>
              </NInput>
              <NSelect
                v-if="protocol === 'http'"
                v-model:value="filters.method"
                size="small"
                clearable
                placeholder="Method"
                :options="HTTP_METHOD_OPTIONS"
                class="ep-filters__select"
              />
              <NSelect
                v-model:value="filters.auth"
                size="small"
                :options="authOptions"
                class="ep-filters__select"
              />
              <NSelect
                v-model:value="filters.status"
                size="small"
                :options="statusOptions"
                class="ep-filters__select"
              />
            </div>

            <NSpin :show="loadingEndpoints">
              <div v-if="filteredEndpoints.length" class="ep-list">
                <button
                  v-for="endpoint in filteredEndpoints"
                  :key="endpoint.id"
                  type="button"
                  class="ep-row"
                  @click="openDetail(endpoint)"
                >
                  <MethodBadge :method="endpoint.http.method" :grpc="endpoint.type === 'grpc'" />
                  <code class="ep-row__path">
                    {{ endpoint.type === "grpc" ? endpoint.grpc.path : endpoint.http.path }}
                  </code>
                  <NIcon
                    v-if="endpoint.auth.enabled"
                    class="ep-row__lock"
                    :component="LockClosedOutline"
                    title="Auth configured"
                  />
                  <span v-if="!endpoint.active" class="ep-row__off">off</span>
                  <span class="ep-row__spacer" />
                  <span class="ep-row__actions" @click.stop>
                    <NButton
                      v-if="endpoint.type === 'http'"
                      quaternary
                      circle
                      size="small"
                      title="Test"
                      @click="openTest(endpoint)"
                    >
                      <NIcon :component="FlashOutline" />
                    </NButton>
                    <NButton
                      v-if="canEdit"
                      quaternary
                      circle
                      size="small"
                      title="Edit"
                      @click="openEdit(endpoint)"
                    >
                      <NIcon :component="CreateOutline" />
                    </NButton>
                    <NButton
                      v-if="canEdit"
                      class="danger-icon-button"
                      quaternary
                      circle
                      size="small"
                      type="error"
                      title="Delete"
                      @click="confirmDeleteEndpoint(endpoint)"
                    >
                      <NIcon :component="TrashOutline" />
                    </NButton>
                  </span>
                </button>
              </div>
              <EmptyState
                v-else
                size="small"
                :description="
                  endpoints.length
                    ? 'No endpoints match the current filters.'
                    : 'No endpoints registered yet.'
                "
              >
                <NButton v-if="canEdit && !endpoints.length" type="primary" @click="openCreate">
                  Create the first endpoint
                </NButton>
              </EmptyState>
            </NSpin>
          </SectionCard>
        </div>

        <EmptyState
          v-else-if="!loadingApp"
          description="Application not found."
        >
          <NButton @click="router.push({ name: 'apps' })">Back to applications</NButton>
        </EmptyState>
      </NSpin>
    </template>

    <!-- Drawers & panels -->
    <EndpointFormDrawer
      v-model:show="showForm"
      :endpoint="editingEndpoint"
      :app="app"
      :prefill="formPrefill"
      @saved="loadEndpoints"
    />
    <EndpointDetailDrawer
      v-model:show="showDetail"
      :endpoint-id="detailId"
      :app="app"
      @edit="onDetailEdit"
      @test="openTest"
      @grpc="showGrpcInstruction = true"
      @changed="loadEndpoints"
    />
    <EndpointTestPanel v-model:show="showTest" :endpoint="testEndpoint" :app="app" />
    <SwaggerSyncPanel
      v-if="app"
      v-model:show="showSwagger"
      :app="app"
      :endpoints="endpoints"
      @changed="loadEndpoints"
    />
    <GrpcReflectionPanel
      v-if="app"
      v-model:show="showGrpcReflection"
      :app="app"
      :endpoints="endpoints"
      @changed="loadEndpoints"
    />
    <GrpcInstructionPanel v-if="app" v-model:show="showGrpcInstruction" :app="app" />
  </PageContainer>
</template>

<style scoped>
.workspace {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.app-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.app-head__title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.app-head__title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
}

.grpc-tag {
  color: var(--c-teal);
  background: rgba(34, 211, 197, 0.14);
}

.app-head__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  font-size: 13px;
  color: var(--c-text-3);
  min-width: 0;
}

.app-head__id {
  color: var(--c-text-2);
}

.app-head__actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.config-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.kv-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.kv-list__row {
  display: grid;
  grid-template-columns: 80px 1fr;
  gap: 12px;
  align-items: center;
}

.kv-list__label {
  font-size: 12px;
  color: var(--c-text-3);
}

.backend-extras {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--c-border);
}

.backend-extras__label {
  display: block;
  font-size: 11.5px;
  margin-bottom: 6px;
}

.ep-toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.protocol-tabs {
  margin-bottom: 12px;
}

.ep-filters {
  display: flex;
  gap: 10px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}

.ep-filters__search {
  max-width: 280px;
}

.ep-filters__select {
  width: 140px;
}

.ep-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.ep-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 9px 12px;
  border: 1px solid var(--c-border);
  border-radius: 9px;
  background: var(--c-surface);
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.14s ease,
    background-color 0.14s ease;
}

.ep-row:hover {
  border-color: var(--c-border-strong);
  background: var(--c-surface-2);
}

.ep-row__path {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--c-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ep-row__lock {
  color: var(--c-text-3);
  flex-shrink: 0;
}

.ep-row__off {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  color: var(--c-warning);
  background: rgba(232, 178, 58, 0.16);
  padding: 1px 6px;
  border-radius: 999px;
}

.ep-row__spacer {
  flex: 1 1 auto;
}

.ep-row__actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

@media (max-width: 920px) {
  .config-grid {
    grid-template-columns: 1fr;
  }
}
</style>
