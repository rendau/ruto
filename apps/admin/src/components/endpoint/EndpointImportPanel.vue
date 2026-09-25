<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import {
  NAlert,
  NButton,
  NCheckbox,
  NCollapse,
  NCollapseItem,
  NEmpty,
  NIcon,
  NInput,
  NModal,
  NSelect,
  NTag,
  useMessage
} from "naive-ui";
import { ClipboardOutline } from "@vicons/ionicons5";
import { createEndpoint } from "@/api/endpoint";
import { apiErrorMessage } from "@/api/http";
import { emptyEndpoint } from "@/lib/entities";
import {
  endpointMatchKey,
  IMPORT_FORMAT_LABELS,
  parseEndpointsText,
  type ParsedEndpoint
} from "@/lib/endpointImport";
import { useClipboard } from "@/composables/useClipboard";
import { HTTP_METHOD_OPTIONS } from "@/constants/enums";
import MethodBadge from "@/components/common/MethodBadge.vue";
import type { AppMain, EndpointMain } from "@/api/types";

type RowStatus = "new" | "exists" | "invalid";

interface Row {
  key: string;
  endpoint: ParsedEndpoint;
  status: RowStatus;
  reason: string;
}

const props = defineProps<{ show: boolean; app: AppMain; endpoints: EndpointMain[] }>();
const emit = defineEmits<{ "update:show": [value: boolean]; changed: [] }>();

const message = useMessage();
const { readSilently } = useClipboard();

const PLACEHOLDER = [
  "Paste endpoints in any format, e.g.",
  "GET /users/{id}",
  "POST https://api.example.com/orders",
  "curl -X DELETE 'https://api.example.com/items/5'",
  "/helloworld.Greeter/SayHello",
  "",
  "…or an OpenAPI/Swagger spec, Postman collection, HAR, .proto file, router code"
].join("\n");

const text = ref("");
const defaultMethod = ref("GET");
const stripPrefix = ref(true);
// Rows the user unchecked; everything new is selected by default.
const excluded = ref<Set<string>>(new Set());
const adding = ref(false);
const inputRef = ref<InstanceType<typeof NInput> | null>(null);

const parsed = computed(() => parseEndpointsText(text.value));

const appPrefix = computed(() => props.app.path_prefix.trim().replace(/^\/+|\/+$/g, ""));

function hasAppPrefix(path: string): boolean {
  const prefix = `/${appPrefix.value}`;
  return appPrefix.value !== "" && (path === prefix || path.startsWith(`${prefix}/`));
}

// Full gateway URLs carry the app path prefix, which endpoint paths must not include.
const prefixedCount = computed(
  () =>
    parsed.value.endpoints.filter((item) => item.type === "http" && hasAppPrefix(item.path))
      .length
);

const registeredKeys = computed(
  () =>
    new Set(
      props.endpoints.map((item) =>
        endpointMatchKey(
          item.type === "grpc"
            ? { type: "grpc", ...item.grpc }
            : { type: "http", method: item.http.method.toUpperCase(), path: `/${item.http.path}` }
        )
      )
    )
);

const rows = computed<Row[]>(() => {
  const seen = new Set<string>();
  const result: Row[] = [];
  for (const item of parsed.value.endpoints) {
    const endpoint = resolve(item);
    const key = endpointMatchKey(endpoint);
    if (seen.has(key)) continue;
    seen.add(key);

    let status: RowStatus = "new";
    let reason = "";
    if (endpoint.type === "http" && endpoint.path.includes("*")) {
      status = "invalid";
      reason = "wildcard '*' is not allowed";
    } else if (registeredKeys.value.has(key)) {
      status = "exists";
      reason = "already registered";
    }
    result.push({ key, endpoint, status, reason });
  }
  return result;
});

const newRows = computed(() => rows.value.filter((row) => row.status === "new"));
const selectedRows = computed(() => newRows.value.filter((row) => !excluded.value.has(row.key)));
const existingCount = computed(() => rows.value.filter((row) => row.status === "exists").length);
const invalidCount = computed(() => rows.value.filter((row) => row.status === "invalid").length);
const hasMissingMethod = computed(() =>
  parsed.value.endpoints.some((item) => item.type === "http" && !item.method)
);

const allSelected = computed(
  () => newRows.value.length > 0 && selectedRows.value.length === newRows.value.length
);
const someSelected = computed(() => selectedRows.value.length > 0 && !allSelected.value);

function resolve(item: ParsedEndpoint): ParsedEndpoint {
  if (item.type === "grpc") return item;
  let path = item.path;
  if (stripPrefix.value && hasAppPrefix(path)) {
    path = path.slice(appPrefix.value.length + 1) || "/";
  }
  return { type: "http", method: item.method || defaultMethod.value, path };
}

function toggle(row: Row): void {
  if (row.status !== "new") return;
  const next = new Set(excluded.value);
  if (next.has(row.key)) {
    next.delete(row.key);
  } else {
    next.add(row.key);
  }
  excluded.value = next;
}

function toggleAll(checked: boolean): void {
  excluded.value = checked ? new Set() : new Set(newRows.value.map((row) => row.key));
}

async function pasteFromClipboard(): Promise<void> {
  const value = await readSilently();
  if (!value.trim()) {
    message.warning("Clipboard is empty or unavailable — paste with Ctrl+V / ⌘V");
    inputRef.value?.focus();
    return;
  }
  text.value = value;
}

async function addSelected(): Promise<void> {
  const chosen = selectedRows.value.map((row) => row.endpoint);
  if (chosen.length === 0) return;
  adding.value = true;
  let created = 0;
  try {
    for (const endpoint of chosen) {
      const draft = emptyEndpoint(props.app.id);
      if (endpoint.type === "grpc") {
        draft.type = "grpc";
        draft.grpc = { service: endpoint.service, method: endpoint.method, path: endpoint.path };
      } else {
        draft.http = { method: endpoint.method, path: endpoint.path };
      }
      await createEndpoint(draft);
      created += 1;
    }
    message.success(`Added ${created} endpoint(s)`);
    emit("changed");
    emit("update:show", false);
  } catch (err) {
    // Already created rows turn into "already registered" once the parent reloads.
    message.error(apiErrorMessage(err, `Added ${created} endpoint(s), then failed`));
    emit("changed");
  } finally {
    adding.value = false;
  }
}

watch(
  () => props.show,
  (show) => {
    if (!show) return;
    text.value = "";
    excluded.value = new Set();
    void nextTick(() => inputRef.value?.focus());
  }
);
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="Import endpoints"
    class="import-modal"
    :bordered="false"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <div class="import">
      <div class="import__source">
        <NInput
          ref="inputRef"
          v-model:value="text"
          type="textarea"
          class="import__textarea"
          :placeholder="PLACEHOLDER"
          :autosize="{ minRows: 6, maxRows: 12 }"
          :input-props="{ spellcheck: false }"
        />
        <div class="import__options">
          <NButton size="small" tertiary @click="pasteFromClipboard">
            <template #icon><NIcon :component="ClipboardOutline" /></template>
            Paste from clipboard
          </NButton>
          <label v-if="hasMissingMethod" class="import__option">
            <span class="muted">Method when not specified</span>
            <NSelect
              v-model:value="defaultMethod"
              size="small"
              class="import__method"
              :options="HTTP_METHOD_OPTIONS"
            />
          </label>
          <NCheckbox v-if="prefixedCount" v-model:checked="stripPrefix" size="small">
            Strip app prefix <code>/{{ appPrefix }}</code> ({{ prefixedCount }})
          </NCheckbox>
        </div>
      </div>

      <section v-if="text.trim()" class="import__section">
        <div class="import__head">
          <NCheckbox
            v-if="newRows.length"
            :checked="allSelected"
            :indeterminate="someSelected"
            @update:checked="toggleAll"
          />
          <span class="section-label">Found {{ rows.length }}</span>
          <span class="muted import__stats">
            {{ newRows.length }} new
            <template v-if="existingCount"> · {{ existingCount }} already registered</template>
            <template v-if="invalidCount"> · {{ invalidCount }} invalid</template>
          </span>
          <div class="import__formats">
            <NTag
              v-for="format in parsed.formats"
              :key="format"
              size="small"
              :bordered="false"
            >
              {{ IMPORT_FORMAT_LABELS[format] }}
            </NTag>
          </div>
        </div>

        <div v-if="rows.length" class="import__list">
          <div
            v-for="row in rows"
            :key="row.key"
            class="import__row"
            :class="`import__row--${row.status}`"
            @click="toggle(row)"
          >
            <NCheckbox
              class="import__check"
              :checked="row.status === 'new' && !excluded.has(row.key)"
              :disabled="row.status !== 'new'"
              :focusable="false"
            />
            <MethodBadge
              :method="row.endpoint.type === 'http' ? row.endpoint.method : ''"
              :grpc="row.endpoint.type === 'grpc'"
            />
            <code class="import__path">{{ row.endpoint.path }}</code>
            <span v-if="row.reason" class="import__reason">{{ row.reason }}</span>
          </div>
        </div>
        <NEmpty v-else size="small" description="No endpoints recognized" />

        <NCollapse v-if="parsed.skipped.length" class="import__skipped">
          <NCollapseItem name="skipped">
            <template #header>
              <span class="muted">Not recognized ({{ parsed.skipped.length }})</span>
            </template>
            <pre class="import__skipped-list">{{ parsed.skipped.join("\n") }}</pre>
          </NCollapseItem>
        </NCollapse>
      </section>

      <NAlert
        v-if="selectedRows.some((row) => row.endpoint.type === 'grpc') && !app.backend.grpc_url"
        type="warning"
        :bordered="false"
      >
        The app has no gRPC backend URL — gRPC endpoints will not be proxied until it is set.
      </NAlert>
    </div>

    <template #footer>
      <div class="import__footer">
        <NButton tertiary @click="emit('update:show', false)">Cancel</NButton>
        <NButton
          type="primary"
          :disabled="selectedRows.length === 0"
          :loading="adding"
          @click="addSelected"
        >
          Add {{ selectedRows.length || "" }} endpoint(s)
        </NButton>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
:global(.import-modal) {
  width: min(820px, calc(100vw - 32px));
}

.import {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.import__source {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.import__textarea :deep(textarea) {
  font-family: var(--font-mono);
  font-size: 12.5px;
}

.import__options {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px 18px;
}

.import__option {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.import__method {
  width: 120px;
}

.import__head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 10px;
  margin-bottom: 10px;
}

.import__stats {
  font-size: 12px;
}

.import__formats {
  display: flex;
  gap: 6px;
  margin-left: auto;
}

.import__list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 320px;
  overflow-y: auto;
}

.import__row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  border: 1px solid var(--c-border);
  border-radius: 8px;
  background: var(--c-surface);
  cursor: pointer;
}

.import__row--exists,
.import__row--invalid {
  cursor: default;
  opacity: 0.6;
}

.import__row--invalid {
  border-color: rgba(232, 178, 58, 0.3);
  background: rgba(232, 178, 58, 0.06);
  opacity: 1;
}

.import__check {
  /* The whole row toggles selection; the checkbox is purely visual. */
  pointer-events: none;
}

.import__path {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--c-text);
  overflow-wrap: anywhere;
}

.import__reason {
  margin-left: auto;
  flex: none;
  font-size: 12px;
  color: var(--c-text-3);
}

.import__skipped {
  margin-top: 12px;
}

.import__skipped-list {
  margin: 0;
  max-height: 160px;
  overflow: auto;
  font-family: var(--font-mono);
  font-size: 12px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.import__footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  width: 100%;
}
</style>
