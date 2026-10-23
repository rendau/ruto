<script setup lang="ts">
import { computed, ref, watch } from "vue";
import {
  NAlert,
  NButton,
  NCheckbox,
  NEmpty,
  NIcon,
  NInput,
  NModal,
  NPopconfirm,
  NSpin,
  NTag,
  useMessage
} from "naive-ui";
import { SearchOutline, TrashOutline } from "@vicons/ionicons5";
import { clearSeenPaths, deleteSeenPaths, listSeenPaths } from "@/api/seenPath";
import { apiErrorMessage } from "@/api/http";
import { formatUnixAge } from "@/lib/datetime";
import { hasWildcard } from "@/lib/endpointPath";
import MethodBadge from "@/components/common/MethodBadge.vue";
import type { AppMain, EndpointMain, SeenPathMain } from "@/api/types";

// Aggregate row the gateway uses once an endpoint exceeds its distinct-path cap.
const OTHER_PATH = "(other)";

// readonly shows the report only: no creating endpoints or deleting rows.
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
const deleting = ref(false);
const deletingKeys = ref(new Set<string>());
const search = ref("");
const error = ref("");
const items = ref<SeenPathMain[]>([]);
const showNotFound = ref(false);
const showRegistered = ref(false);

// Without a tracked wildcard the report stays empty, which is easy to mistake
// for no traffic.
const hasTrackedWildcard = computed(() =>
  props.endpoints.some(
    (item) => item.type === "http" && item.active && item.track_paths && hasWildcard(item.http.path)
  )
);

function splitPath(path: string): string[] {
  return path.replace(/^\/+|\/+$/g, "").split("/");
}

const explicitEndpoints = computed(() =>
  props.endpoints
    .filter((item) => item.type === "http" && !item.http.path.includes("*"))
    .map((item) => ({ method: item.http.method.toUpperCase(), segments: splitPath(item.http.path) }))
);

// The gateway collapses only numbers, uuids, hashes and long slugs to {id}, so
// a code like "AB123" stays literal: a path param of an endpoint has to match
// any segment, not just {id}. An endpoint may serve one method or any ("*").
function isRegistered(item: SeenPathMain): boolean {
  const segments = splitPath(item.path);
  return explicitEndpoints.value.some(
    (endpoint) =>
      (endpoint.method === item.method || endpoint.method === "*") &&
      endpoint.segments.length === segments.length &&
      endpoint.segments.every(
        (segment, index) => /^(\{.+\}|:.+)$/.test(segment) || segment === segments[index]
      )
  );
}

function rowsLabel(count: number): string {
  return `${count} row${count === 1 ? "" : "s"}`;
}

function rowKey(item: SeenPathMain): string {
  return `${item.endpoint_id} ${item.method} ${item.path}`;
}

const rows = computed(() =>
  items.value.map((item) => ({
    item,
    key: rowKey(item),
    registered: item.path !== OTHER_PATH && isRegistered(item),
    onlyNotFound: item.hits > 0 && item.hits_not_found >= item.hits,
    other: item.path === OTHER_PATH
  }))
);

const searchText = computed(() => search.value.trim().replace(/^\/+/, "").toLowerCase());

const matchedRows = computed(() =>
  searchText.value
    ? rows.value.filter((row) => row.item.path.toLowerCase().includes(searchText.value))
    : rows.value
);

const visibleRows = computed(() =>
  matchedRows.value.filter(
    (row) => (showNotFound.value || !row.onlyNotFound) && (showRegistered.value || !row.registered)
  )
);

const notFoundCount = computed(() => matchedRows.value.filter((row) => row.onlyNotFound).length);
const registeredItems = computed(() =>
  rows.value.filter((row) => row.registered).map((row) => row.item)
);
const registeredCount = computed(
  () => matchedRows.value.filter((row) => row.registered).length
);

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

// Rows reappear on their own if the path is still served, so deleting needs no
// confirmation; bulk deletes still ask.
async function remove(list: SeenPathMain[]): Promise<boolean> {
  if (!list.length) return false;
  const keys = list.map(rowKey);
  keys.forEach((key) => deletingKeys.value.add(key));
  try {
    await deleteSeenPaths(
      props.app.id,
      list.map((item) => ({ endpoint_id: item.endpoint_id, method: item.method, path: item.path }))
    );
    const removed = new Set(keys);
    items.value = items.value.filter((item) => !removed.has(rowKey(item)));
    return true;
  } catch (err) {
    message.error(apiErrorMessage(err, "Failed to delete seen paths"));
    return false;
  } finally {
    keys.forEach((key) => deletingKeys.value.delete(key));
  }
}

async function removeBulk(list: SeenPathMain[]): Promise<void> {
  deleting.value = true;
  try {
    const count = list.length;
    if (await remove(list)) message.success(`Deleted ${rowsLabel(count)}`);
  } finally {
    deleting.value = false;
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
        What wildcard endpoints of this app with <b>Track seen paths</b> on actually serve. Identifiers
        are collapsed to <code>{id}</code>. Register an explicit endpoint for a path and it stops being
        counted here. Gateways report about every 30 seconds.
      </p>
      <NAlert v-if="!hasTrackedWildcard" type="info" :bordered="false" class="seen__alert">
        No active wildcard endpoint of this app has <b>Track seen paths</b> on, so new requests are
        not counted. Turn it on in the endpoint form.
      </NAlert>

      <div class="seen__filters">
        <NInput
          v-model:value="search"
          size="small"
          clearable
          placeholder="Filter by path, e.g. product/"
          class="seen__search"
        >
          <template #prefix><NIcon :component="SearchOutline" /></template>
        </NInput>
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
          <NButton
            v-if="!readonly"
            size="tiny"
            quaternary
            circle
            type="error"
            title="Delete this row"
            aria-label="Delete this row"
            :loading="deletingKeys.has(row.key)"
            @click="remove([row.item])"
          >
            <template #icon><NIcon :component="TrashOutline" /></template>
          </NButton>
        </div>
      </div>
      <NEmpty
        v-else
        size="small"
        :description="
          !items.length
            ? 'No requests seen under wildcard endpoints yet'
            : searchText && !matchedRows.length
              ? 'No paths match the filter'
              : 'Nothing left to register'
        "
      />
    </NSpin>

    <template #footer>
      <div class="seen__footer">
        <NButton tertiary :loading="loading" @click="load">Refresh</NButton>
        <div v-if="!readonly" class="seen__footer-actions">
          <NPopconfirm v-if="registeredItems.length" @positive-click="removeBulk(registeredItems)">
            <template #trigger>
              <NButton tertiary type="warning" :loading="deleting">
                Delete registered ({{ registeredItems.length }})
              </NButton>
            </template>
            <div class="seen__confirm">
              Delete {{ rowsLabel(registeredItems.length) }} of paths that already have explicit
              endpoints? They no longer get counted, but still take up the per-endpoint limit.
            </div>
          </NPopconfirm>
          <NPopconfirm
            v-if="searchText && matchedRows.length"
            @positive-click="removeBulk(matchedRows.map((row) => row.item))"
          >
            <template #trigger>
              <NButton tertiary type="warning" :loading="deleting">
                Delete matching ({{ matchedRows.length }})
              </NButton>
            </template>
            <div class="seen__confirm">
              Delete {{ rowsLabel(matchedRows.length) }} whose path contains “{{ searchText }}”,
              including hidden ones?
            </div>
          </NPopconfirm>
          <NPopconfirm @positive-click="clear">
            <template #trigger>
              <NButton tertiary type="error" :disabled="!items.length" :loading="clearing">Clear all</NButton>
            </template>
            Delete all seen paths of this app and start counting from scratch?
          </NPopconfirm>
        </div>
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
  align-items: center;
  gap: 6px 18px;
  margin-bottom: 12px;
}

.seen__search {
  width: 260px;
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
  flex-wrap: wrap;
  gap: 10px;
  width: 100%;
}

.seen__confirm {
  max-width: 320px;
}

.seen__footer-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
