<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { NAlert, NButton, NCheckbox, NEmpty, NModal, NPopconfirm, NSpin, NTag, useMessage } from "naive-ui";
import { clearSeenPaths, listSeenPaths } from "@/api/seenPath";
import { apiErrorMessage } from "@/api/http";
import { endpointMatchKey } from "@/lib/endpointImport";
import { formatUnixAge } from "@/lib/datetime";
import MethodBadge from "@/components/common/MethodBadge.vue";
import type { AppMain, EndpointMain, SeenPathMain } from "@/api/types";

// Aggregate row the gateway uses once an endpoint exceeds its distinct-path cap.
const OTHER_PATH = "(other)";

// readonly shows the report only: no creating endpoints or clearing.
const props = defineProps<{
  show: boolean;
  app: AppMain;
  endpoints: EndpointMain[];
  readonly?: boolean;
}>();
const emit = defineEmits<{
  "update:show": [value: boolean];
  create: [prefill: Partial<EndpointMain>];
}>();

const message = useMessage();

const loading = ref(false);
const clearing = ref(false);
const error = ref("");
const items = ref<SeenPathMain[]>([]);
const showNotFound = ref(false);
const showRegistered = ref(false);

const registeredKeys = computed(
  () =>
    new Set(
      props.endpoints
        .filter((item) => item.type === "http" && !item.http.path.includes("*"))
        .map((item) =>
          endpointMatchKey({
            type: "http",
            method: item.http.method.toUpperCase(),
            path: `/${item.http.path}`
          })
        )
    )
);

// An explicit endpoint may be registered for one method or for any ("*").
function isRegistered(item: SeenPathMain): boolean {
  const path = `/${item.path}`;
  return (
    registeredKeys.value.has(endpointMatchKey({ type: "http", method: item.method, path })) ||
    registeredKeys.value.has(endpointMatchKey({ type: "http", method: "*", path }))
  );
}

const rows = computed(() =>
  items.value.map((item) => ({
    item,
    key: `${item.endpoint_id} ${item.method} ${item.path}`,
    registered: isRegistered(item),
    onlyNotFound: item.hits > 0 && item.hits_not_found >= item.hits,
    other: item.path === OTHER_PATH
  }))
);

const visibleRows = computed(() =>
  rows.value.filter(
    (row) => (showNotFound.value || !row.onlyNotFound) && (showRegistered.value || !row.registered)
  )
);

const notFoundCount = computed(() => rows.value.filter((row) => row.onlyNotFound).length);
const registeredCount = computed(() => rows.value.filter((row) => row.registered).length);

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    items.value = (await listSeenPaths(props.app.id)).results;
  } catch (err) {
    error.value = apiErrorMessage(err, "Failed to load seen paths");
  } finally {
    loading.value = false;
  }
}

async function clear(): Promise<void> {
  clearing.value = true;
  try {
    await clearSeenPaths(props.app.id);
    items.value = [];
    message.success("Seen paths cleared");
  } catch (err) {
    message.error(apiErrorMessage(err, "Failed to clear seen paths"));
  } finally {
    clearing.value = false;
  }
}

function create(item: SeenPathMain): void {
  emit("create", { type: "http", http: { method: item.method, path: `/${item.path}` } });
  emit("update:show", false);
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
    title="Seen paths"
    class="seen-modal"
    :bordered="false"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <NSpin :show="loading">
      <NAlert v-if="error" type="error" :bordered="false" class="seen__alert">{{ error }}</NAlert>

      <p class="muted seen__sub">
        What wildcard endpoints of this app actually serve. Identifiers are collapsed to
        <code>{id}</code>. Register an explicit endpoint for a path and it stops being counted here.
        Gateways report about every 30 seconds.
      </p>

      <div class="seen__filters">
        <NCheckbox v-model:checked="showNotFound">
          Show paths that only returned 404 ({{ notFoundCount }})
        </NCheckbox>
        <NCheckbox v-model:checked="showRegistered">
          Show already registered ({{ registeredCount }})
        </NCheckbox>
      </div>

      <div v-if="visibleRows.length" class="seen__list">
        <div v-for="row in visibleRows" :key="row.key" class="seen__row">
          <MethodBadge :method="row.item.method" />
          <div class="seen__main">
            <code class="seen__path">{{ row.other ? row.item.path : `/${row.item.path}` }}</code>
            <span v-if="row.other" class="muted seen__sample">
              too many distinct paths — the rest is counted together
            </span>
            <span v-else-if="row.item.sample" class="muted seen__sample" :title="row.item.sample">
              e.g. {{ row.item.sample }}
            </span>
          </div>
          <NTag v-if="row.registered" size="small" type="success" :bordered="false">registered</NTag>
          <NTag v-else-if="row.onlyNotFound" size="small" type="warning" :bordered="false">404 only</NTag>
          <span class="seen__hits mono" :title="`${row.item.hits_not_found} of them returned 404`">
            {{ row.item.hits.toLocaleString() }}
          </span>
          <span class="seen__age muted">{{ formatUnixAge(row.item.last_seen_at_unix) }}</span>
          <NButton
            v-if="!readonly && !row.registered && !row.other"
            size="tiny"
            tertiary
            type="primary"
            @click="create(row.item)"
          >
            Create endpoint
          </NButton>
        </div>
      </div>
      <NEmpty
        v-else
        size="small"
        :description="items.length ? 'Nothing left to register' : 'No requests seen under wildcard endpoints yet'"
      />
    </NSpin>

    <template #footer>
      <div class="seen__footer">
        <NButton tertiary :loading="loading" @click="load">Refresh</NButton>
        <NPopconfirm v-if="!readonly" @positive-click="clear">
          <template #trigger>
            <NButton tertiary type="error" :disabled="!items.length" :loading="clearing">Clear</NButton>
          </template>
          Delete all seen paths of this app and start counting from scratch?
        </NPopconfirm>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
:global(.seen-modal) {
  width: min(860px, calc(100vw - 48px));
}

.seen__alert {
  margin-bottom: 14px;
}

.seen__sub {
  margin: 0 0 12px;
  font-size: 12px;
}

.seen__filters {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 18px;
  margin-bottom: 12px;
}

.seen__list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 460px;
  overflow-y: auto;
}

.seen__row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  border: 1px solid var(--c-border);
  border-radius: 8px;
  background: var(--c-surface);
}

.seen__main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1;
}

.seen__path {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--c-text);
  overflow-wrap: anywhere;
}

.seen__sample {
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.seen__hits {
  flex: none;
  min-width: 56px;
  text-align: right;
  font-size: 13px;
}

.seen__age {
  flex: none;
  min-width: 64px;
  text-align: right;
  font-size: 12px;
}

.seen__footer {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  width: 100%;
}
</style>
