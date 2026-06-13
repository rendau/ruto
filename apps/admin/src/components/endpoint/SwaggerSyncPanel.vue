<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { NAlert, NButton, NCheckbox, NEmpty, NModal, NSpin, useMessage } from "naive-ui";
import { getAppSwaggerEndpointsDiff } from "@/api/app";
import { createEndpoint } from "@/api/endpoint";
import { apiErrorMessage } from "@/api/http";
import { emptyEndpoint } from "@/lib/entities";
import MethodBadge from "@/components/common/MethodBadge.vue";
import type { AppMain, AppSwaggerEndpoint, EndpointMain } from "@/api/types";

const props = defineProps<{ show: boolean; app: AppMain; endpoints: EndpointMain[] }>();
const emit = defineEmits<{ "update:show": [value: boolean]; changed: [] }>();

const message = useMessage();

const loading = ref(false);
const error = ref("");
const unregistered = ref<AppSwaggerEndpoint[]>([]);
const registeredInvalid = ref<AppSwaggerEndpoint[]>([]);
const selected = ref<Set<string>>(new Set());
const adding = ref(false);

const allSelected = computed(
  () => unregistered.value.length > 0 && selected.value.size === unregistered.value.length
);

function keyOf(endpoint: AppSwaggerEndpoint): string {
  return `${endpoint.method} ${endpoint.path}`;
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
  const key = keyOf(endpoint);
  const next = new Set(selected.value);
  if (next.has(key)) {
    next.delete(key);
  } else {
    next.add(key);
  }
  selected.value = next;
}

function toggleAll(value: boolean): void {
  selected.value = value ? new Set(unregistered.value.map(keyOf)) : new Set();
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
    await load();
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
    title="Swagger sync"
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
            <NCheckbox
              v-if="unregistered.length"
              :checked="allSelected"
              @update:checked="toggleAll"
            >
              Select all
            </NCheckbox>
          </div>
          <div v-if="unregistered.length" class="swagger__list">
            <label
              v-for="endpoint in unregistered"
              :key="keyOf(endpoint)"
              class="swagger__row"
            >
              <NCheckbox
                :checked="selected.has(keyOf(endpoint))"
                @update:checked="() => toggle(endpoint)"
              />
              <MethodBadge :method="endpoint.method" />
              <code class="swagger__path">{{ endpoint.path }}</code>
            </label>
          </div>
          <NEmpty v-else size="small" description="Everything is in sync" />
        </section>

        <section v-if="registeredInvalid.length" class="swagger__section">
          <span class="section-label">Registered but missing in swagger</span>
          <p class="muted swagger__sub">These endpoints are no longer present in the swagger spec</p>
          <div class="swagger__list">
            <div
              v-for="endpoint in registeredInvalid"
              :key="keyOf(endpoint)"
              class="swagger__row swagger__row--invalid"
            >
              <MethodBadge :method="endpoint.method" />
              <code class="swagger__path">{{ endpoint.path }}</code>
            </div>
          </div>
        </section>
      </div>
    </NSpin>

    <template #footer>
      <div class="swagger__footer">
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

.swagger__row--invalid {
  cursor: default;
  border-color: rgba(232, 178, 58, 0.3);
  background: rgba(232, 178, 58, 0.06);
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
