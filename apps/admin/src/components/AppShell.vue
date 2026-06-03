<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";
import { NIcon, useDialog, type DropdownOption } from "naive-ui";
import {
  AddOutline,
  LogOutOutline,
  MenuOutline,
  PeopleOutline,
  PersonOutline,
  RocketOutline,
  SearchOutline,
  ServerOutline,
  SettingsOutline,
  SyncOutline
} from "@vicons/ionicons5";
import { useAuthStore } from "../stores/auth";
import { useAppsStore } from "../stores/apps";
import { ApiError, deploySnapshot } from "../lib/api";
import { notifyError, notifySuccess } from "../lib/notify";

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const appsStore = useAppsStore();
const dialog = useDialog();
const layoutRef = ref<HTMLElement | null>(null);
const mobileSidebarOpen = ref(false);
const deploying = ref(false);
const sidebarWidth = ref(290);
const minSidebarWidth = 220;
const maxSidebarWidth = 420;
let resizing = false;

const profileName = computed(() => authStore.profile?.name || authStore.profile?.username || "Unknown");
const profileInitial = computed(() => profileName.value.trim().charAt(0).toUpperCase() || "U");
const profileRole = computed(() => (authStore.profile?.is_admin ? "admin" : "user"));
const canManageUsers = computed(() => Boolean(authStore.profile?.is_admin));
const appSearch = ref("");
const userMenuOptions = computed<DropdownOption[]>(() => [
  { label: "Profile", key: "profile", icon: () => h(NIcon, null, { default: () => h(PersonOutline) }) },
  { label: "Logout", key: "logout", icon: () => h(NIcon, null, { default: () => h(LogOutOutline) }) }
]);
const userMenuProps = () => ({ class: "user-dropdown-menu" });
const pageTitle = computed(() => {
  switch (route.name) {
    case "dashboard":
      return "Dashboard";
    case "app-create":
      return "Create Application";
    case "app-details":
      return "Application";
    case "app-edit":
      return "Edit Application";
    case "endpoint-create":
      return "Create Endpoint";
    case "endpoint-details":
      return "Endpoint";
    case "endpoint-edit":
      return "Edit Endpoint";
    case "root-edit":
      return "Root Settings";
    case "profile":
      return "Profile";
    case "gateways":
      return "Gateways";
    case "gateway-details":
      return "Gateway";
    case "users":
      return "Users";
    case "users-create":
      return "Create User";
    case "users-edit":
      return "Edit User";
    default:
      return "Control Panel";
  }
});

const filteredApps = computed(() => {
  const query = appSearch.value.trim().toLowerCase();
  if (!query) {
    return appsStore.items;
  }
  return appsStore.items.filter((app) => {
    const haystack = [app.name, app.id, app.path_prefix, app.backend?.url].join(" ").toLowerCase();
    return haystack.includes(query);
  });
});

async function reloadMenuApps() {
  try {
    await appsStore.loadMenuApps();
  } catch {
    appsStore.items = [];
  }
}

function logout() {
  dialog.warning({
    title: "Log out",
    content: "Log out from current session?",
    positiveText: "Log out",
    negativeText: "Cancel",
    onPositiveClick: () => {
      authStore.logout();
      void router.push({ name: "login" });
    }
  });
}

function goToProfile() {
  router.push({ name: "profile" });
}

function onEscKey(event: KeyboardEvent) {
  if (event.key === "Escape") {
    mobileSidebarOpen.value = false;
  }
}

function onMouseMove(event: MouseEvent) {
  if (!resizing || !layoutRef.value) {
    return;
  }
  const rect = layoutRef.value.getBoundingClientRect();
  const nextWidth = Math.round(event.clientX - rect.left);
  sidebarWidth.value = Math.min(maxSidebarWidth, Math.max(minSidebarWidth, nextWidth));
}

function stopResize() {
  if (!resizing) {
    return;
  }
  resizing = false;
  localStorage.setItem("ruto_admin_sidebar_width", String(sidebarWidth.value));
  document.body.style.userSelect = "";
  document.removeEventListener("mousemove", onMouseMove);
  document.removeEventListener("mouseup", stopResize);
}

function startResize(event: MouseEvent) {
  event.preventDefault();
  resizing = true;
  document.body.style.userSelect = "none";
  document.addEventListener("mousemove", onMouseMove);
  document.addEventListener("mouseup", stopResize);
}

function openMobileSidebar() {
  mobileSidebarOpen.value = true;
}

function closeMobileSidebar() {
  mobileSidebarOpen.value = false;
}

async function deploy() {
  if (deploying.value) {
    return;
  }
  dialog.info({
    title: "Deploy snapshot",
    content: "Start deploy snapshot to gateways?",
    positiveText: "Deploy",
    negativeText: "Cancel",
    onPositiveClick: () => {
      void runDeploy();
    }
  });
}

async function runDeploy() {
  deploying.value = true;
  try {
    await deploySnapshot();
    notifySuccess("Deploy started");
  } catch (error) {
    if (error instanceof ApiError) {
      notifyError(error.message);
    } else {
      notifyError("Unable to start deploy");
    }
  } finally {
    deploying.value = false;
  }
}

function handleUserMenuSelect(key: string | number) {
  if (key === "profile") {
    goToProfile();
  }
  if (key === "logout") {
    logout();
  }
}

function displayBackendUrl(url?: string): string {
  if (!url) {
    return "-";
  }
  return url.replace(/^https?:\/\//i, "");
}

onMounted(() => {
  const stored = Number(localStorage.getItem("ruto_admin_sidebar_width") || "");
  if (Number.isFinite(stored) && stored >= minSidebarWidth && stored <= maxSidebarWidth) {
    sidebarWidth.value = stored;
  }
  void reloadMenuApps();
  document.addEventListener("keydown", onEscKey);
});

watch(
  () => route.fullPath,
  () => {
    mobileSidebarOpen.value = false;
  }
);

onBeforeUnmount(() => {
  stopResize();
  document.removeEventListener("keydown", onEscKey);
});
</script>

<template>
  <div ref="layoutRef" class="layout" :style="{ '--sidebar-width': `${sidebarWidth}px` }">
    <aside class="sidebar" :class="{ 'mobile-open': mobileSidebarOpen }">
      <RouterLink class="brand brand-link" to="/">
        <img class="brand-logo" src="/logo-ruto.svg" alt="Ruto logo" />
        <span class="brand-text">Ruto Admin</span>
      </RouterLink>
      <nav class="nav">
        <div class="nav-row">
          <RouterLink class="nav-link with-icon" to="/root/edit">
            <span class="nav-link-content">
              <n-icon class="icon" :component="SettingsOutline" aria-hidden="true" />
              <span>Root Settings</span>
            </span>
          </RouterLink>
          <n-button
            class="nav-icon-button"
            attr-type="button"
            size="small"
            secondary
            :disabled="deploying"
            title="Deploy"
            aria-label="Deploy"
            @click="deploy"
          >
            <n-icon :component="deploying ? SyncOutline : RocketOutline" aria-hidden="true" />
          </n-button>
        </div>
        <RouterLink v-if="canManageUsers" class="nav-link with-icon" to="/users">
          <span class="nav-link-content">
            <n-icon class="icon users-icon" :component="PeopleOutline" aria-hidden="true" />
            <span>Users</span>
          </span>
        </RouterLink>
        <RouterLink class="nav-link with-icon" to="/gateways">
          <span class="nav-link-content">
            <n-icon class="icon" :component="ServerOutline" aria-hidden="true" />
            <span>Gateways</span>
          </span>
        </RouterLink>
      </nav>

      <div class="menu-block-head">
        <div class="menu-block-title">Apps</div>
        <RouterLink class="apps-create-icon" to="/apps/new" title="Create App" aria-label="Create App">
          <n-icon :component="AddOutline" aria-hidden="true" />
        </RouterLink>
      </div>
      <div class="apps-search-wrap">
        <n-input v-model:value="appSearch" class="apps-search" type="text" size="small" placeholder="Search apps" aria-label="Search apps" clearable>
          <template #prefix>
            <n-icon :component="SearchOutline" />
          </template>
        </n-input>
      </div>
      <div class="apps-list">
        <RouterLink
          v-for="app in filteredApps"
          :key="app.id"
          :to="{ name: 'app-details', params: { id: app.id } }"
          class="app-item"
          :class="{ active: route.params.id === app.id }"
          :title="`${app.name || app.id} (${app.path_prefix || '/'})`"
        >
          <span class="app-topline">
            <span class="app-name">{{ app.name || app.id }}</span>
            <span class="app-prefix">{{ app.path_prefix || "/" }}</span>
          </span>
          <span class="app-backend" :title="app.backend?.url || '-'">
            {{ displayBackendUrl(app.backend?.url) }}
          </span>
        </RouterLink>
        <div v-if="filteredApps.length === 0" class="apps-empty muted">No apps found</div>
      </div>
    </aside>
    <button
      class="sidebar-resize"
      type="button"
      title="Resize sidebar"
      aria-label="Resize sidebar"
      @mousedown="startResize"
    ></button>
    <button v-if="mobileSidebarOpen" class="sidebar-overlay" type="button" @click="closeMobileSidebar"></button>

    <main class="content">
      <header class="topbar">
        <div class="topbar-left">
          <n-button class="mobile-menu-button" attr-type="button" size="small" secondary @click="openMobileSidebar" aria-label="Open menu">
            <n-icon :component="MenuOutline" />
          </n-button>
          <div class="topbar-title">{{ pageTitle }}</div>
        </div>
        <n-dropdown trigger="click" :options="userMenuOptions" :menu-props="userMenuProps" @select="handleUserMenuSelect">
          <button class="user-trigger" type="button">
            <span class="avatar-badge" aria-hidden="true">{{ profileInitial }}</span>
            <span class="user-name">{{ profileName }}</span>
            <n-tag size="small" :type="authStore.profile?.is_admin ? 'success' : 'default'">{{ profileRole }}</n-tag>
            <span class="icon caret" aria-hidden="true">▾</span>
          </button>
        </n-dropdown>
      </header>
      <section class="page">
        <RouterView v-slot="{ Component, route: currentRoute }">
          <Transition name="page-fade-slide" mode="out-in">
            <div :key="currentRoute.fullPath" class="page-transition-wrap">
              <component :is="Component" />
            </div>
          </Transition>
        </RouterView>
      </section>
    </main>
  </div>
</template>
