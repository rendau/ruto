<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { NAlert, NButton, NCheckbox, NEmpty, NModal, NSpin, useMessage } from "naive-ui";
import { getAppGrpcReflectionEndpoints } from "@/api/app";
import { createEndpoint } from "@/api/endpoint";
import { apiErrorMessage } from "@/api/http";
import { emptyEndpoint } from "@/lib/entities";
import MethodBadge from "@/components/common/MethodBadge.vue";
import type { AppGrpcReflectionEndpoint, AppMain, EndpointMain } from "@/api/types";

const props = defineProps<{ show: boolean; app: AppMain; endpoints: EndpointMain[] }>();
const emit = defineEmits<{ "update:show": [value: boolean]; changed: [] }>();

const message = useMessage();

const loading = ref(false);
const error = ref("");
const results = ref<AppGrpcReflectionEndpoint[]>([]);
const selected = ref<Set<string>>(new Set());
const adding = ref(false);

const registeredPaths = computed(
  () => new Set(props.endpoints.filter((e) => e.type === "grpc").map((e) => e.grpc.path))
);

const unregistered = computed(() =>
  results.value.filter((endpoint) => !registeredPaths.value.has(endpoint.path))
);

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  selected.value = new Set();
  try {
    const rep = await getAppGrpcReflectionEndpoints(props.app.id);
    results.value = rep.results ?? [];
  } catch (err) {
    error.value = apiErrorMessage(err, "Failed to fetch gRPC reflection");
  } finally {
    loading.value = false;
  }
}

function toggle(path: string): void {
  const next = new Set(selected.value);
  if (next.has(path)) {
    next.delete(path);
  } else {
    next.add(path);
  }
  selected.value = next;
}

async function addSelected(): Promise<void> {
  const chosen = unregistered.value.filter((endpoint) => selected.value.has(endpoint.path));
  if (chosen.length === 0) return;
  adding.value = true;
  let created = 0;
  try {
    for (const endpoint of chosen) {
      const draft = emptyEndpoint(props.app.id);
      draft.type = "grpc";
      draft.grpc = { service: endpoint.service, method: endpoint.method, path: endpoint.path };
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
    title="gRPC reflection"
    class="reflection-modal"
    :bordered="false"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <NSpin :show="loading">
      <NAlert v-if="error" type="error" :bordered="false" class="reflection__alert">
        {{ error }}
      </NAlert>

      <div class="reflection__head">
        <div>
          <span class="section-label">Unregistered methods</span>
          <p class="muted reflection__sub">
            Discovered via reflection but not registered in ruto
          </p>
        </div>
      </div>

      <div v-if="unregistered.length" class="reflection__list">
        <label v-for="endpoint in unregistered" :key="endpoint.path" class="reflection__row">
          <NCheckbox
            :checked="selected.has(endpoint.path)"
            @update:checked="() => toggle(endpoint.path)"
          />
          <MethodBadge grpc method="grpc" />
          <div class="reflection__info">
            <code class="reflection__path">{{ endpoint.path }}</code>
            <span class="muted reflection__svc">{{ endpoint.service }} · {{ endpoint.method }}</span>
          </div>
        </label>
      </div>
      <NEmpty
        v-else-if="!loading"
        size="small"
        :description="results.length ? 'All reflected methods are registered' : 'No methods reflected'"
      />
    </NSpin>

    <template #footer>
      <div class="reflection__footer">
        <NButton tertiary :loading="loading" @click="load">Refresh</NButton>
        <NButton
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
:global(.reflection-modal) {
  width: min(720px, calc(100vw - 48px));
}

.reflection__alert {
  margin-bottom: 14px;
}

.reflection__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.reflection__sub {
  margin: 2px 0 0;
  font-size: 12px;
}

.reflection__list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 360px;
  overflow-y: auto;
}

.reflection__row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  border: 1px solid var(--c-border);
  border-radius: 8px;
  background: var(--c-surface);
  cursor: pointer;
}

.reflection__info {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.reflection__path {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--c-text);
  overflow-wrap: anywhere;
}

.reflection__svc {
  font-size: 11.5px;
}

.reflection__footer {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  width: 100%;
}
</style>
