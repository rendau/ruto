<script setup lang="ts">
import { ref, watch } from "vue";
import {
  NAlert,
  NButton,
  NCheckbox,
  NCollapse,
  NCollapseItem,
  NEmpty,
  NIcon,
  NModal,
  NPopconfirm,
  NSpin,
  useMessage
} from "naive-ui";
import { TrashOutline } from "@vicons/ionicons5";
import { getAppSwaggerEndpointsDiff } from "@/api/app";
import { createEndpoint, deleteEndpoint } from "@/api/endpoint";
import { apiErrorMessage } from "@/api/http";
import { emptyEndpoint } from "@/lib/entities";
import MethodBadge from "@/components/common/MethodBadge.vue";
import type { AppMain, AppSwaggerEndpoint, EndpointMain } from "@/api/types";

// readonly shows the diff only: no selection, adding or deleting (for users
// who can view the app but not manage it).
const props = defineProps<{
  show: boolean;
  app: AppMain;
  endpoints: EndpointMain[];
  readonly?: boolean;
}>();
const emit = defineEmits<{ "update:show": [value: boolean]; changed: [] }>();

const message = useMessage();

const loading = ref(false);
const error = ref("");
const unregistered = ref<AppSwaggerEndpoint[]>([]);
const registeredInvalid = ref<AppSwaggerEndpoint[]>([]);
const selected = ref<Set<string>>(new Set());
const adding = ref(false);
const deletingKey = ref("");

function keyOf(endpoint: AppSwaggerEndpoint): string {
  return `${endpoint.method} ${endpoint.path}`;
}

// The diff returns endpoints normalized (upper-cased method, single leading slash),
// so registered endpoints must be normalized the same way to be matched back by id.
function normalizedKeyOf(method: string, path: string): string {
  const p = path.trim().replace(/^\/+|\/+$/g, "");
  return `${method.trim().toUpperCase()} /${p}`;
}

function findRegisteredEndpoint(endpoint: AppSwaggerEndpoint): EndpointMain | undefined {
  const key = keyOf(endpoint);
  return props.endpoints.find(
    (item) => item.type === "http" && normalizedKeyOf(item.http.method, item.http.path) === key
  );
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  selected.value = new Set();
  try {
    const rep = await getAppSwaggerEndpointsDiff(props.app.id);
    unregistered.value = rep.unregistered ?? [];
    registeredInvalid.value = rep.registered_invalid ?? [];
  } catch (err) {
    error.value = apiErrorMessage(err, "Failed to fetch swagger endpoints");
  } finally {
    loading.value = false;
  }
}

function toggle(endpoint: AppSwaggerEndpoint): void {
  if (props.readonly) return;
  const key = keyOf(endpoint);
  const next = new Set(selected.value);
  if (next.has(key)) {
    next.delete(key);
  } else {
    next.add(key);
  }
  selected.value = next;
}

async function addSelected(): Promise<void> {
  const chosen = unregistered.value.filter((endpoint) => selected.value.has(keyOf(endpoint)));
  if (chosen.length === 0) return;
  adding.value = true;
  let created = 0;
  try {
    for (const endpoint of chosen) {
      const draft = emptyEndpoint(props.app.id);
      draft.type = "http";
      draft.http = { method: endpoint.method, path: endpoint.path };
      await createEndpoint(draft);
      created += 1;
    }
    message.success(`Added ${created} endpoint(s)`);
    emit("changed");
    emit("update:show", false);
  } catch (err) {
    message.error(apiErrorMessage(err, `Added ${created} endpoint(s), then failed`));
    emit("changed");
  } finally {
    adding.value = false;
  }
}

async function removeRegisteredInvalid(endpoint: AppSwaggerEndpoint): Promise<void> {
  const registered = findRegisteredEndpoint(endpoint);
  if (!registered) {
    message.error("Endpoint not found, refresh the list");
    return;
  }
  deletingKey.value = keyOf(endpoint);
  try {
    await deleteEndpoint(registered.id);
    registeredInvalid.value = registeredInvalid.value.filter(
      (item) => keyOf(item) !== keyOf(endpoint)
    );
    message.success("Endpoint deleted");
    emit("changed");
  } catch (err) {
    message.error(apiErrorMessage(err, "Failed to delete endpoint"));
  } finally {
    deletingKey.value = "";
  }
}

watch(
  () => props.show,
  (show) => {
    if (show) void load();
  }
);
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="Swagger"
    class="swagger-modal"
    :bordered="false"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <NSpin :show="loading">
      <NAlert v-if="error" type="error" :bordered="false" class="swagger__alert">{{ error }}</NAlert>

      <div class="swagger">
        <section class="swagger__section">
          <div class="swagger__head">
            <div>
              <span class="section-label">Unregistered</span>
              <p class="muted swagger__sub">In swagger but not registered in ruto</p>
            </div>
          </div>
          <div v-if="unregistered.length" class="swagger__list">
            <div
              v-for="endpoint in unregistered"
              :key="keyOf(endpoint)"
              class="swagger__row"
              :class="{ 'swagger__row--readonly': readonly }"
              @click="toggle(endpoint)"
            >
              <NCheckbox
                v-if="!readonly"
                class="swagger__check"
                :checked="selected.has(keyOf(endpoint))"
                :focusable="false"
              />
              <MethodBadge :method="endpoint.method" />
              <code class="swagger__path">{{ endpoint.path }}</code>
            </div>
          </div>
          <NEmpty v-else size="small" description="Everything is in sync" />
        </section>

        <section v-if="registeredInvalid.length" class="swagger__section">
          <NCollapse>
            <NCollapseItem name="invalid">
              <template #header>
                <span class="section-label">
                  Registered but missing in swagger ({{ registeredInvalid.length }})
                </span>
              </template>
              <p class="muted swagger__sub">
                These endpoints are no longer present in the swagger spec
              </p>
              <div class="swagger__list">
                <div
                  v-for="endpoint in registeredInvalid"
                  :key="keyOf(endpoint)"
                  class="swagger__row swagger__row--invalid"
                >
                  <MethodBadge :method="endpoint.method" />
                  <code class="swagger__path">{{ endpoint.path }}</code>
                  <NPopconfirm
                    v-if="!readonly"
                    @positive-click="removeRegisteredInvalid(endpoint)"
                  >
                    <template #trigger>
                      <NButton
                        class="danger-icon-button swagger__delete"
                        quaternary
                        circle
                        size="small"
                        type="error"
                        title="Delete"
                        :loading="deletingKey === keyOf(endpoint)"
                      >
                        <NIcon :component="TrashOutline" />
                      </NButton>
                    </template>
                    Delete endpoint "{{ endpoint.method }} {{ endpoint.path }}"?
                  </NPopconfirm>
                </div>
              </div>
            </NCollapseItem>
          </NCollapse>
        </section>
      </div>
    </NSpin>

    <template #footer>
      <div class="swagger__footer">
        <NButton tertiary :loading="loading" @click="load">Refresh</NButton>
        <NButton
          v-if="!readonly"
          type="primary"
          :disabled="selected.size === 0"
          :loading="adding"
          @click="addSelected"
        >
          Add {{ selected.size || "" }} selected
        </NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
:global(.swagger-modal) {
  width: min(720px, calc(100vw - 48px));
}

.swagger {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.swagger__alert {
  margin-bottom: 14px;
}

.swagger__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.swagger__sub {
  margin: 2px 0 0;
  font-size: 12px;
}

.swagger__list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 320px;
  overflow-y: auto;
}

.swagger__row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  border: 1px solid var(--c-border);
  border-radius: 8px;
  background: var(--c-surface);
  cursor: pointer;
}

.swagger__row--readonly {
  cursor: default;
}

.swagger__check {
  /* The whole row toggles selection; the checkbox is purely visual. */
  pointer-events: none;
}

.swagger__row--invalid {
  cursor: default;
  border-color: rgba(232, 178, 58, 0.3);
  background: rgba(232, 178, 58, 0.06);
}

.swagger__delete {
  margin-left: auto;
  flex: none;
}

.swagger__path {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--c-text);
  overflow-wrap: anywhere;
}

.swagger__footer {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  width: 100%;
}
</style>
