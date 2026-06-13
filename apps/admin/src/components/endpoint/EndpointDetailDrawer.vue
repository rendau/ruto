<script setup lang="ts">
import { computed, ref, watch } from "vue";
import {
  NButton,
  NDrawer,
  NDrawerContent,
  NIcon,
  NSpin,
  NSwitch,
  NTabPane,
  NTabs,
  useMessage
} from "naive-ui";
import {
  CreateOutline,
  FlashOutline,
  TerminalOutline,
  TrashOutline
} from "@vicons/ionicons5";
import {
  deleteEndpoint,
  getEndpoint,
  getEndpointInherited,
  getEndpointInterpolate,
  updateEndpoint
} from "@/api/endpoint";
import { apiErrorMessage } from "@/api/http";
import { useDrawerResource } from "@/composables/useDrawerResource";
import { useConfirm } from "@/composables/useConfirm";
import { useIsMobile } from "@/composables/useIsMobile";
import { useRootStore } from "@/stores/root";
import { variablesToArray } from "@/api/normalize";
import { joinPath, joinUrl } from "@/lib/format";
import MethodBadge from "@/components/common/MethodBadge.vue";
import StatusTag from "@/components/common/StatusTag.vue";
import CopyText from "@/components/common/CopyText.vue";
import DefList from "@/components/common/DefList.vue";
import DefRow from "@/components/common/DefRow.vue";
import SwitchField from "@/components/common/SwitchField.vue";
import KeyValueGrid from "@/components/common/KeyValueGrid.vue";
import AuthSummary from "@/components/display/AuthSummary.vue";
import LoggingSummary from "@/components/display/LoggingSummary.vue";
import type { AppMain, EndpointMain, Variable } from "@/api/types";

const props = defineProps<{
  show: boolean;
  endpointId: string | null;
  app: AppMain | null;
}>();

const emit = defineEmits<{
  "update:show": [value: boolean];
  edit: [endpoint: EndpointMain];
  test: [endpoint: EndpointMain];
  grpc: [];
  changed: [];
}>();

const message = useMessage();
const rootStore = useRootStore();
const { confirmDelete } = useConfirm();
const isMobile = useIsMobile();

const inherited = ref<EndpointMain | null>(null);
const interpolated = ref<EndpointMain | null>(null);
const showInterpolated = ref(false);
const toggling = ref(false);

const { loading, item, reload } = useDrawerResource<EndpointMain, string>({
  show: () => props.show,
  id: () => props.endpointId,
  fetch: getEndpoint,
  onLoaded: async (endpoint) => {
    inherited.value = null;
    interpolated.value = null;
    showInterpolated.value = false;
    try {
      inherited.value = await getEndpointInherited({
        id: endpoint.id,
        app_id: endpoint.app_id,
        variables: []
      });
    } catch {
      inherited.value = endpoint;
    }
  },
  onError: () => emit("update:show", false)
});

// The "Effective" tab shows the fully inherited endpoint; toggling "Interpolate"
// swaps in the variant with variables resolved (lazily fetched on first use).
const effective = computed<EndpointMain | null>(() => {
  if (showInterpolated.value) {
    return interpolated.value ?? inherited.value ?? item.value;
  }
  return inherited.value ?? item.value;
});

function hasBackendExtras(ep: EndpointMain | null): boolean {
  const backend = ep?.backend;
  return Boolean(
    backend &&
      (Object.keys(backend.headers || {}).length || Object.keys(backend.query_params || {}).length)
  );
}

function toItems(record: Record<string, string> | undefined): Variable[] {
  return variablesToArray(record);
}

const publicUrl = computed(() => {
  const endpoint = item.value;
  if (!endpoint || endpoint.type !== "http" || !props.app) return "";
  const path = joinPath(props.app.path_prefix, endpoint.http.path);
  return rootStore.baseUrl ? joinUrl(rootStore.baseUrl, path) : path;
});

watch(showInterpolated, async (value) => {
  if (value && !interpolated.value && item.value) {
    try {
      interpolated.value = await getEndpointInterpolate({
        id: item.value.id,
        app_id: item.value.app_id,
        variables: []
      });
    } catch (error) {
      message.error(apiErrorMessage(error, "Failed to interpolate"));
      showInterpolated.value = false;
    }
  }
});

async function toggleActive(): Promise<void> {
  if (!item.value) return;
  toggling.value = true;
  try {
    await updateEndpoint({ ...item.value, active: !item.value.active });
    message.success(item.value.active ? "Endpoint deactivated" : "Endpoint activated");
    await reload();
    emit("changed");
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to update endpoint"));
  } finally {
    toggling.value = false;
  }
}

function remove(): void {
  const endpoint = item.value;
  if (!endpoint) return;
  confirmDelete({
    content: `Delete endpoint "${endpoint.id}"? This cannot be undone.`,
    onConfirm: async () => {
      try {
        await deleteEndpoint(endpoint.id);
        message.success("Endpoint deleted");
        emit("update:show", false);
        emit("changed");
      } catch (error) {
        message.error(apiErrorMessage(error, "Failed to delete endpoint"));
      }
    }
  });
}
</script>

<template>
  <NDrawer
    :show="show"
    :width="isMobile ? '100%' : 600"
    placement="right"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <NDrawerContent title="Endpoint" closable>
      <NSpin :show="loading">
        <div v-if="item" class="detail">
          <div class="detail__head">
            <MethodBadge
              :method="item.http.method"
              :grpc="item.type === 'grpc'"
            />
            <code class="detail__path">
              {{ item.type === "grpc" ? item.grpc.path : item.http.path }}
            </code>
            <StatusTag :active="item.active" />
          </div>

          <DefList>
            <DefRow label="Id">
              <CopyText :value="item.id" label="Endpoint id" wrap />
            </DefRow>
            <DefRow v-if="item.type === 'grpc'" label="Service">
              <span class="mono">{{ item.grpc.service }}</span>
            </DefRow>
            <DefRow v-if="item.type === 'grpc'" label="Method">
              <span class="mono">{{ item.grpc.method }}</span>
            </DefRow>
            <DefRow v-if="publicUrl" label="Public URL">
              <CopyText :value="publicUrl" label="Public URL" wrap />
            </DefRow>
            <DefRow v-if="item.backend.custom_path" label="Backend path">
              <span class="mono">{{ item.backend.custom_path }}</span>
            </DefRow>
            <DefRow label="Metrics">
              {{ item.exclude_from_metrics ? "Excluded" : "Included" }}
            </DefRow>
          </DefList>

          <section class="detail__section">
            <NTabs type="segment" size="small" animated>
              <NTabPane name="configured" tab="Configured">
                <div class="detail__stack">
                  <div v-if="hasBackendExtras(item)">
                    <span class="section-label">Backend extras</span>
                    <div class="detail__cols">
                      <div>
                        <span class="muted detail__minilabel">Headers</span>
                        <KeyValueGrid
                          :items="toItems(item.backend.headers)"
                          empty-text="No headers"
                        />
                      </div>
                      <div>
                        <span class="muted detail__minilabel">Query params</span>
                        <KeyValueGrid
                          :items="toItems(item.backend.query_params)"
                          empty-text="No query params"
                        />
                      </div>
                    </div>
                  </div>
                  <div>
                    <span class="section-label">Authentication</span>
                    <AuthSummary :auth="item.auth" />
                  </div>
                  <div>
                    <span class="section-label">Logging</span>
                    <LoggingSummary :logging="item.logging" />
                  </div>
                  <div>
                    <span class="section-label">Variables</span>
                    <KeyValueGrid :items="item.variables" empty-text="No variables" />
                  </div>
                </div>
              </NTabPane>
              <NTabPane name="effective" tab="Effective">
                <div v-if="effective" class="detail__stack">
                  <div class="detail__vars-head">
                    <span class="muted detail__hint">Inherited from app &amp; root</span>
                    <SwitchField v-model="showInterpolated" label="Interpolate" />
                  </div>
                  <div v-if="hasBackendExtras(effective)">
                    <span class="section-label">Backend extras</span>
                    <div class="detail__cols">
                      <div>
                        <span class="muted detail__minilabel">Headers</span>
                        <KeyValueGrid
                          :items="toItems(effective.backend.headers)"
                          empty-text="No headers"
                        />
                      </div>
                      <div>
                        <span class="muted detail__minilabel">Query params</span>
                        <KeyValueGrid
                          :items="toItems(effective.backend.query_params)"
                          empty-text="No query params"
                        />
                      </div>
                    </div>
                  </div>
                  <div>
                    <span class="section-label">Authentication</span>
                    <AuthSummary :auth="effective.auth" />
                  </div>
                  <div>
                    <span class="section-label">Logging</span>
                    <LoggingSummary :logging="effective.logging" />
                  </div>
                  <div>
                    <span class="section-label">Variables</span>
                    <KeyValueGrid :items="effective.variables" empty-text="No variables" />
                  </div>
                </div>
              </NTabPane>
            </NTabs>
          </section>
        </div>
      </NSpin>

      <template #footer>
        <div class="detail__footer">
          <NButton
            class="danger-icon-button"
            type="error"
            tertiary
            :disabled="!item"
            @click="remove"
          >
            <template #icon><NIcon :component="TrashOutline" /></template>
            Delete
          </NButton>
          <div class="detail__footer-right">
            <label v-if="item" class="switch-field">
              <NSwitch
                :value="item.active"
                :loading="toggling"
                size="small"
                @update:value="toggleActive"
              />
              <span class="switch-field__label">Active</span>
            </label>
            <NButton
              v-if="item && item.type === 'http'"
              tertiary
              :disabled="!item"
              @click="item && emit('test', item)"
            >
              <template #icon><NIcon :component="FlashOutline" /></template>
              Test
            </NButton>
            <NButton
              v-if="item && item.type === 'grpc'"
              tertiary
              @click="emit('grpc')"
            >
              <template #icon><NIcon :component="TerminalOutline" /></template>
              Connect
            </NButton>
            <NButton type="primary" :disabled="!item" @click="item && emit('edit', item)">
              <template #icon><NIcon :component="CreateOutline" /></template>
              Edit
            </NButton>
          </div>
        </div>
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.detail {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.detail__head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.detail__path {
  font-family: var(--font-mono);
  font-size: 14px;
  color: var(--c-text);
  overflow-wrap: anywhere;
}

.detail__section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.detail__cols {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.detail__minilabel {
  display: block;
  font-size: 11.5px;
  margin-bottom: 6px;
}

.detail__stack {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-top: 6px;
}

.detail__vars-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.detail__hint {
  font-size: 12px;
}

.detail__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  flex-wrap: wrap;
}

.detail__footer-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

@media (max-width: 520px) {
  .detail__cols {
    grid-template-columns: 1fr;
  }
}
</style>
