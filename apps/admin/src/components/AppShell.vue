<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";
import { useAuthStore } from "../stores/auth";
import { useAppsStore } from "../stores/apps";
import { ApiError, deploySnapshot } from "../lib/api";
import { notifyError, notifySuccess } from "../lib/notify";

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const appsStore = useAppsStore();
const layoutRef = ref<HTMLElement | null>(null);
const userMenuOpen = ref(false);
const userMenuRef = ref<HTMLElement | null>(null);
const mobileSidebarOpen = ref(false);
const deploying = ref(false);
const sidebarWidth = ref(290);
const minSidebarWidth = 220;
const maxSidebarWidth = 420;
let resizing = false;

const profileName = computed(() => authStore.profile?.name || authStore.profile?.username || "Unknown");
const profileInitial = computed(() => profileName.value.trim().charAt(0).toUpperCase() || "U");
const profileRole = computed(() => (authStore.profile?.is_admin ? "admin" : "user"));
const appSearch = ref("");
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
    case "endpoint-edit":
      return "Edit Endpoint";
    case "root-edit":
      return "Root Settings";
    case "profile":
      return "Profile";
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
  const approved = window.confirm("Log out from current session?");
  if (!approved) {
    return;
  }
  userMenuOpen.value = false;
  authStore.logout();
  router.push({ name: "login" });
}

function goToProfile() {
  userMenuOpen.value = false;
  router.push({ name: "profile" });
}

function toggleUserMenu() {
  userMenuOpen.value = !userMenuOpen.value;
}

function onDocumentClick(event: MouseEvent) {
  if (!userMenuRef.value) {
    return;
  }
  const target = event.target as Node | null;
  if (target && !userMenuRef.value.contains(target)) {
    userMenuOpen.value = false;
  }
}

function onEscKey(event: KeyboardEvent) {
  if (event.key === "Escape") {
    userMenuOpen.value = false;
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
  document.addEventListener("click", onDocumentClick);
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
  document.removeEventListener("click", onDocumentClick);
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
              <span class="icon" aria-hidden="true">⚙</span>
              <span>Root Settings</span>
            </span>
          </RouterLink>
          <button
            class="nav-icon-button"
            type="button"
            :disabled="deploying"
            title="Deploy"
            aria-label="Deploy"
            @click="deploy"
          >
            <span aria-hidden="true">{{ deploying ? "⏳" : "🚀" }}</span>
          </button>
        </div>
      </nav>

      <div class="menu-block-head">
        <div class="menu-block-title">Apps</div>
        <RouterLink class="apps-create-icon" to="/apps/new" title="Create App" aria-label="Create App">
          <span aria-hidden="true">＋</span>
        </RouterLink>
      </div>
      <div class="apps-search-wrap">
        <input v-model="appSearch" class="apps-search" type="search" placeholder="Search apps" aria-label="Search apps" />
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
          <button class="mobile-menu-button" type="button" @click="openMobileSidebar" aria-label="Open menu">☰</button>
          <div class="topbar-title">{{ pageTitle }}</div>
        </div>
        <div class="topbar-user" ref="userMenuRef">
          <button class="user-trigger" type="button" @click="toggleUserMenu">
            <span class="avatar-badge" aria-hidden="true">{{ profileInitial }}</span>
            <span>{{ profileName }}</span>
            <span class="admin-badge" :class="{ yes: authStore.profile?.is_admin }">{{ profileRole }}</span>
            <span class="icon caret" aria-hidden="true">▾</span>
          </button>
          <div v-if="userMenuOpen" class="user-menu">
            <button class="menu-item" type="button" @click="goToProfile">
              <span class="icon" aria-hidden="true">🧑</span>
              <span>Profile</span>
            </button>
            <button class="menu-item danger" type="button" @click="logout">
              <span class="icon" aria-hidden="true">⎋</span>
              <span>Logout</span>
            </button>
          </div>
        </div>
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
