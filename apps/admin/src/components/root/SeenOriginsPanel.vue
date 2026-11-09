<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { NAlert, NButton, NCheckbox, NIcon, NPopconfirm, NSpin, NTag, useMessage } from "naive-ui";
import { AddOutline, RefreshOutline, TrashOutline } from "@vicons/ionicons5";
import { clearSeenOrigins, deleteSeenOrigins, listSeenOrigins } from "@/api/seenOrigin";
import { apiErrorMessage } from "@/api/http";
import { formatUnixAge } from "@/lib/datetime";
import { originAllowed } from "@/lib/origins";
import type { SeenOriginMain } from "@/api/types";

// allowOrigins is the list as currently edited (unsaved lines included), so a
// just-added origin turns "allowed" at once.
const props = defineProps<{
  allowOrigins: string[];
  corsEnabled: boolean;
  readonly?: boolean;
}>();
const emit = defineEmits<{ add: [origin: string] }>();

const message = useMessage();

const loading = ref(false);
const clearing = ref(false);
const deletingKeys = ref(new Set<string>());
const error = ref("");
const items = ref<SeenOriginMain[]>([]);
const showAllowed = ref(false);

const rows = computed(() =>
  items.value.map((item) => ({ item, allowed: originAllowed(item.origin, props.allowOrigins) }))
);
const allowedCount = computed(() => rows.value.filter((row) => row.allowed).length);
const visibleRows = computed(() => rows.value.filter((row) => showAllowed.value || !row.allowed));

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    items.value = (await listSeenOrigins()).results;
  } catch (err) {
    error.value = apiErrorMessage(err, "Failed to load seen origins");
  } finally {
    loading.value = false;
  }
}

async function clear(): Promise<void> {
  clearing.value = true;
  try {
    await clearSeenOrigins();
    items.value = [];
    message.success("Seen origins cleared");
  } catch (err) {
    message.error(apiErrorMessage(err, "Failed to clear seen origins"));
  } finally {
    clearing.value = false;
  }
}

// A row reappears on its own while the front keeps calling, so deleting one
// needs no confirmation.
async function remove(item: SeenOriginMain): Promise<void> {
  deletingKeys.value.add(item.origin);
  try {
    await deleteSeenOrigins([item.origin]);
    items.value = items.value.filter((row) => row.origin !== item.origin);
  } catch (err) {
    message.error(apiErrorMessage(err, "Failed to delete the origin"));
  } finally {
    deletingKeys.value.delete(item.origin);
  }
}

onMounted(load);
</script>

<template>
  <div class="origins">
    <div class="origins__head">
      <div class="origins__heading">
        <span class="origins__title">Seen origins</span>
        <p class="muted origins__sub">
          Where browsers call the gateway from (the <code>Origin</code> header), whether CORS admits them
          or not. Add one to <b>Allow origins</b> with a click, then save. Gateways report about every
          30 seconds.
        </p>
      </div>
      <div class="origins__actions">
        <NButton size="tiny" quaternary :loading="loading" @click="load">
          <template #icon><NIcon :component="RefreshOutline" /></template>
          Refresh
        </NButton>
        <NPopconfirm v-if="!readonly && items.length" @positive-click="clear">
          <template #trigger>
            <NButton size="tiny" quaternary type="error" :loading="clearing">
              <template #icon><NIcon :component="TrashOutline" /></template>
              Clear
            </NButton>
          </template>
          <div class="origins__confirm">
            Forget every seen origin? Counters start over from the next gateway report.
          </div>
        </NPopconfirm>
      </div>
    </div>

    <NAlert v-if="error" type="error" :bordered="false" class="origins__alert">{{ error }}</NAlert>

    <NSpin :show="loading">
      <div v-if="rows.length" class="origins__filters">
        <NCheckbox v-model:checked="showAllowed">Show already allowed ({{ allowedCount }})</NCheckbox>
      </div>

      <div v-if="visibleRows.length" class="origins__list">
        <div v-for="row in visibleRows" :key="row.item.origin" class="origins__row">
          <code class="origins__origin">{{ row.item.origin }}</code>
          <NTag v-if="row.allowed" size="small" type="success" :bordered="false">allowed</NTag>
          <NTag v-else-if="corsEnabled" size="small" type="warning" :bordered="false">not allowed</NTag>
          <span class="origins__hits mono" title="Requests with this origin">
            {{ row.item.hits.toLocaleString() }}
          </span>
          <span class="origins__age muted">{{ formatUnixAge(row.item.last_seen_at_unix) }}</span>
          <NButton
            v-if="!readonly && !row.allowed"
            size="tiny"
            tertiary
            type="primary"
            @click="emit('add', row.item.origin)"
          >
            <template #icon><NIcon :component="AddOutline" /></template>
            Add
          </NButton>
          <NButton
            v-if="!readonly"
            size="tiny"
            quaternary
            circle
            type="error"
            title="Forget this origin"
            aria-label="Forget this origin"
            :loading="deletingKeys.has(row.item.origin)"
            @click="remove(row.item)"
          >
            <template #icon><NIcon :component="TrashOutline" /></template>
          </NButton>
        </div>
      </div>
      <p v-else-if="!loading && !error" class="muted origins__empty">
        {{
          rows.length
            ? "Every seen origin is already allowed."
            : "No origins seen yet: no browser has called the gateway since the last clear."
        }}
      </p>
    </NSpin>
  </div>
</template>

<style scoped>
.origins {
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid var(--c-border);
}

.origins__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px 14px;
  margin-bottom: 10px;
}

.origins__heading {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1 1 320px;
}

.origins__title {
  font-size: 14px;
  font-weight: 600;
}

.origins__sub {
  margin: 0;
  font-size: 12px;
}

.origins__actions {
  display: flex;
  gap: 6px;
  flex: none;
}

.origins__alert {
  margin-bottom: 10px;
}

.origins__filters {
  margin-bottom: 8px;
}

.origins__list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 320px;
  overflow-y: auto;
}

.origins__row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 10px;
  border: 1px solid var(--c-border);
  border-radius: 8px;
  background: var(--c-surface);
}

.origins__origin {
  flex: 1;
  min-width: 0;
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--c-text);
  overflow-wrap: anywhere;
}

.origins__hits {
  flex: none;
  min-width: 48px;
  text-align: right;
  font-size: 13px;
}

.origins__age {
  flex: none;
  min-width: 64px;
  text-align: right;
  font-size: 12px;
}

.origins__confirm {
  max-width: 300px;
}

.origins__empty {
  margin: 0;
  font-size: 12px;
}
</style>
