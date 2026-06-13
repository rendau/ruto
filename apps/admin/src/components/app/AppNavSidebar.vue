<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { storeToRefs } from "pinia";
import { NButton, NIcon, NInput, NScrollbar, NSpin } from "naive-ui";
import { AddOutline, SearchOutline } from "@vicons/ionicons5";
import { useAppsStore } from "@/stores/apps";
import { useAppForm } from "@/composables/useAppForm";
import { useAuthStore } from "@/stores/auth";
import { stripScheme } from "@/lib/format";
import type { AppMain } from "@/api/types";

const emit = defineEmits<{ navigate: [] }>();

const route = useRoute();
const appsStore = useAppsStore();
const authStore = useAuthStore();
const { apps, loading } = storeToRefs(appsStore);
const appForm = useAppForm();

const search = ref("");

const currentId = computed(() => (typeof route.params.id === "string" ? route.params.id : null));

const filtered = computed<AppMain[]>(() => {
  const query = search.value.trim().toLowerCase();
  if (!query) {
    return apps.value;
  }
  return apps.value.filter((app) =>
    [app.name, app.id, app.path_prefix, app.backend?.url].join(" ").toLowerCase().includes(query)
  );
});

function hasGrpc(app: AppMain): boolean {
  return Boolean((app.backend?.grpc_url || "").trim());
}

onMounted(() => {
  void appsStore.ensureLoaded();
});
</script>

<template>
  <div class="app-nav">
    <div class="app-nav__head">
      <span class="app-nav__title section-label">Applications</span>
      <NButton
        v-if="authStore.isAdmin"
        size="tiny"
        type="primary"
        secondary
        @click="appForm.open(null)"
      >
        <template #icon><NIcon :component="AddOutline" /></template>
        New
      </NButton>
    </div>

    <div class="app-nav__search">
      <NInput v-model:value="search" size="small" placeholder="Search applications" clearable>
        <template #prefix><NIcon :component="SearchOutline" /></template>
      </NInput>
    </div>

    <NScrollbar class="app-nav__list">
      <NSpin :show="loading" size="small">
        <div v-if="!loading && filtered.length === 0" class="app-nav__empty muted">
          No applications
        </div>
        <RouterLink
          v-for="app in filtered"
          :key="app.id"
          :to="{ name: 'app-workspace', params: { id: app.id } }"
          class="app-item"
          :class="{ 'app-item--active': app.id === currentId }"
          @click="emit('navigate')"
        >
          <span class="app-item__top">
            <span class="app-item__name">{{ app.name || app.id }}</span>
            <span class="app-item__badges">
              <span v-if="!app.active" class="app-item__badge app-item__badge--off">off</span>
              <span v-if="hasGrpc(app)" class="app-item__badge app-item__badge--grpc">gRPC</span>
            </span>
          </span>
          <span class="app-item__meta">
            <span class="app-item__prefix mono">{{ app.path_prefix || "/" }}</span>
            <span class="app-item__backend">{{ stripScheme(app.backend?.url) || "—" }}</span>
          </span>
        </RouterLink>
      </NSpin>
    </NScrollbar>
  </div>
</template>

<style scoped>
.app-nav {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.app-nav__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 16px 8px;
}

.app-nav__search {
  padding: 0 12px 10px;
}

.app-nav__list {
  flex: 1 1 auto;
  min-height: 0;
}

.app-nav__empty {
  padding: 24px 16px;
  text-align: center;
  font-size: 13px;
}

.app-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 9px 12px;
  margin: 2px 8px;
  border-radius: 9px;
  border: 1px solid transparent;
  transition:
    background-color 0.14s ease,
    border-color 0.14s ease;
}

.app-item:hover {
  background: rgba(255, 255, 255, 0.03);
}

.app-item--active {
  background: var(--c-primary-soft);
  border-color: rgba(91, 130, 240, 0.35);
}

.app-item__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.app-item__name {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--c-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-item__badges {
  display: inline-flex;
  gap: 5px;
  flex-shrink: 0;
}

.app-item__badge {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  padding: 1px 6px;
  border-radius: 999px;
  line-height: 1.5;
}

.app-item__badge--off {
  background: rgba(232, 178, 58, 0.16);
  color: var(--c-warning);
}

.app-item__badge--grpc {
  background: rgba(34, 211, 197, 0.14);
  color: var(--c-teal);
}

.app-item__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11.5px;
  color: var(--c-text-3);
  overflow: hidden;
}

.app-item__prefix {
  color: var(--c-text-2);
  flex-shrink: 0;
}

.app-item__backend {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
