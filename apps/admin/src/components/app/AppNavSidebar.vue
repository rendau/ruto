<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { storeToRefs } from "pinia";
import { NButton, NIcon, NInput, NScrollbar, NSpin } from "naive-ui";
import {
  AddOutline,
  ChevronDownOutline,
  ChevronForwardOutline,
  FolderOpenOutline,
  FolderOutline,
  SearchOutline
} from "@vicons/ionicons5";
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

const COLLAPSED_STORAGE_KEY = "ruto_admin_app_groups_collapsed";
const UNGROUPED = "";

interface AppGroup {
  name: string;
  apps: AppMain[];
}

const search = ref("");
const scope = ref<"mine" | "all">("mine");
const collapsed = ref<Set<string>>(readCollapsed());

function readCollapsed(): Set<string> {
  try {
    const raw = JSON.parse(localStorage.getItem(COLLAPSED_STORAGE_KEY) || "[]");
    return new Set(Array.isArray(raw) ? raw.filter((v) => typeof v === "string") : []);
  } catch {
    return new Set();
  }
}

function storeCollapsed(): void {
  localStorage.setItem(COLLAPSED_STORAGE_KEY, JSON.stringify(Array.from(collapsed.value)));
}

const currentId = computed(() => (typeof route.params.id === "string" ? route.params.id : null));

// Users with full app access manage every app, so "mine" == "all" for them and
// the scope filter is hidden.
const showScopeFilter = computed(() => !authStore.hasFullAppAccess);

const filtered = computed<AppMain[]>(() => {
  let list = apps.value;
  if (showScopeFilter.value && scope.value === "mine") {
    list = list.filter((app) => authStore.canManageApp(app.id));
  }
  const query = search.value.trim().toLowerCase();
  if (!query) {
    return list;
  }
  return list.filter((app) =>
    [app.name, app.id, app.path_prefix, app.group, app.backend?.url]
      .join(" ")
      .toLowerCase()
      .includes(query)
  );
});

// Named groups sorted alphabetically, ungrouped apps last. Headers are only
// worth drawing once at least one app has a group.
const groups = computed<AppGroup[]>(() => {
  const byName = new Map<string, AppMain[]>();
  for (const app of filtered.value) {
    const name = app.group.trim();
    const bucket = byName.get(name);
    if (bucket) {
      bucket.push(app);
    } else {
      byName.set(name, [app]);
    }
  }
  const named = Array.from(byName.entries())
    .filter(([name]) => name !== UNGROUPED)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([name, items]) => ({ name, apps: items }));
  const ungrouped = byName.get(UNGROUPED);
  if (ungrouped) {
    named.push({ name: UNGROUPED, apps: ungrouped });
  }
  return named;
});

const hasGroups = computed(() => apps.value.some((app) => app.group.trim() !== UNGROUPED));

function isCollapsed(group: AppGroup): boolean {
  // A search query always shows every match, otherwise collapsed groups would
  // hide hits the user is looking for.
  return !search.value.trim() && collapsed.value.has(group.name);
}

function toggleGroup(group: AppGroup): void {
  const next = new Set(collapsed.value);
  if (next.has(group.name)) {
    next.delete(group.name);
  } else {
    next.add(group.name);
  }
  collapsed.value = next;
  storeCollapsed();
}

function expandGroupOf(appId: string | null): void {
  if (!appId) return;
  const app = appsStore.getById(appId);
  if (!app) return;
  const name = app.group.trim();
  if (!collapsed.value.has(name)) return;
  const next = new Set(collapsed.value);
  next.delete(name);
  collapsed.value = next;
  storeCollapsed();
}

// Never leave the currently opened app hidden inside a collapsed group, also
// after the list is reloaded (e.g. the app was just moved to another group).
watch([currentId, apps], ([id]) => expandGroupOf(id), { immediate: true });

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

    <div v-if="showScopeFilter" class="app-nav__scope">
      <button
        type="button"
        class="scope-btn"
        :class="{ 'scope-btn--active': scope === 'mine' }"
        @click="scope = 'mine'"
      >
        My apps
      </button>
      <button
        type="button"
        class="scope-btn"
        :class="{ 'scope-btn--active': scope === 'all' }"
        @click="scope = 'all'"
      >
        All apps
      </button>
    </div>

    <NScrollbar class="app-nav__list">
      <NSpin :show="loading" size="small">
        <div v-if="!loading && filtered.length === 0" class="app-nav__empty muted">
          No applications
        </div>
        <div
          v-for="group in groups"
          :key="group.name || '__ungrouped'"
          class="app-group"
          :class="{
            'app-group--named': hasGroups && group.name,
            'app-group--collapsed': isCollapsed(group)
          }"
        >
          <button
            v-if="hasGroups && group.name"
            type="button"
            class="app-group__head"
            @click="toggleGroup(group)"
          >
            <NIcon
              class="app-group__folder"
              :component="isCollapsed(group) ? FolderOutline : FolderOpenOutline"
            />
            <span class="app-group__name">{{ group.name }}</span>
            <span class="app-group__count">{{ group.apps.length }}</span>
            <NIcon
              class="app-group__chevron"
              :component="isCollapsed(group) ? ChevronForwardOutline : ChevronDownOutline"
            />
          </button>
          <div v-else-if="hasGroups" class="app-group__divider">
            <span>Ungrouped</span>
          </div>
          <div v-if="!isCollapsed(group)" class="app-group__body">
            <RouterLink
              v-for="app in group.apps"
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
          </div>
        </div>
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

.app-nav__scope {
  display: flex;
  gap: 4px;
  padding: 0 12px 10px;
}

.scope-btn {
  flex: 1 1 0;
  padding: 5px 8px;
  border-radius: 7px;
  border: 1px solid var(--c-border);
  background: transparent;
  color: var(--c-text-3);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition:
    background-color 0.14s ease,
    border-color 0.14s ease,
    color 0.14s ease;
}

.scope-btn:hover {
  color: var(--c-text);
  background: rgba(255, 255, 255, 0.03);
}

.scope-btn--active {
  color: var(--c-text);
  background: var(--c-primary-soft);
  border-color: rgba(99, 226, 183, 0.32);
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

/* Named groups are drawn as folders: a bordered card with a tinted header
   row, so they read as containers rather than as one more app row. */
.app-group--named {
  margin: 6px 8px 0;
  border: 1px solid var(--c-border);
  border-radius: 11px;
  background: rgba(255, 255, 255, 0.02);
  overflow: hidden;
}

.app-group__head {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 10px 8px 12px;
  border: 0;
  border-bottom: 1px solid var(--c-border);
  background: rgba(99, 226, 183, 0.07);
  color: var(--c-text);
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  text-align: left;
  cursor: pointer;
  transition: background-color 0.14s ease;
}

.app-group__head:hover {
  background: rgba(99, 226, 183, 0.12);
}

.app-group--collapsed .app-group__head {
  border-bottom-color: transparent;
  background: rgba(255, 255, 255, 0.04);
  color: var(--c-text-2);
}

.app-group--collapsed .app-group__head:hover {
  background: rgba(255, 255, 255, 0.07);
}

.app-group__folder {
  flex-shrink: 0;
  font-size: 16px;
  color: var(--c-primary);
}

.app-group--collapsed .app-group__folder {
  color: var(--c-text-3);
}

.app-group__chevron {
  flex-shrink: 0;
  font-size: 13px;
  color: var(--c-text-3);
}

.app-group__name {
  flex: 1 1 auto;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-group__count {
  flex-shrink: 0;
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0;
  color: var(--c-text-2);
  background: rgba(255, 255, 255, 0.07);
  border-radius: 999px;
  padding: 0 7px;
  line-height: 1.6;
}

.app-group--named .app-group__body {
  padding: 4px;
}

.app-group--named .app-item {
  margin: 2px 0;
}

/* Ungrouped apps stay a flat list under a quiet divider label. */
.app-group__divider {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 14px 16px 4px;
  font-size: 10.5px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--c-text-3);
}

.app-group__divider::after {
  content: "";
  flex: 1 1 auto;
  height: 1px;
  background: var(--c-border);
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
  border-color: rgba(99, 226, 183, 0.32);
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
