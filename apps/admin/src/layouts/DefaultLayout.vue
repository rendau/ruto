<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, watch } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";
import { NButton, NDropdown, NIcon, NTag, useMessage, type DropdownOption } from "naive-ui";
import {
  LogOutOutline,
  MenuOutline,
  PersonOutline,
  RocketOutline,
  SyncOutline
} from "@vicons/ionicons5";
import { storeToRefs } from "pinia";
import { useAuthStore } from "@/stores/auth";
import { useUiStore, SIDEBAR_MAX_WIDTH, SIDEBAR_MIN_WIDTH } from "@/stores/ui";
import { useSnapshotStore } from "@/stores/snapshot";
import { useConfirm } from "@/composables/useConfirm";
import { apiErrorMessage } from "@/api/http";
import BrandLogo from "@/components/common/BrandLogo.vue";
import AppNavSidebar from "@/components/app/AppNavSidebar.vue";
import AppFormDrawer from "@/components/app/AppFormDrawer.vue";

const route = useRoute();
const router = useRouter();
const message = useMessage();
const authStore = useAuthStore();
const uiStore = useUiStore();
const snapshotStore = useSnapshotStore();
const { confirmAction } = useConfirm();
const { sidebarWidth, mobileSidebarOpen } = storeToRefs(uiStore);
const { version, deploying } = storeToRefs(snapshotStore);

const profileName = computed(
  () => authStore.profile?.name || authStore.profile?.username || "Unknown"
);
const profileInitial = computed(() => profileName.value.trim().charAt(0).toUpperCase() || "U");

const navLinks = computed(() => {
  const links = [
    { name: "dashboard", label: "Dashboard" },
    { name: "gateways", label: "Gateways" },
    { name: "root-config", label: "Root config" }
  ];
  if (authStore.isAdmin) {
    links.push({ name: "users", label: "Users" });
  }
  return links;
});

const activeNav = computed(() => route.name);

const shortVersion = computed(() => (version.value ? version.value.slice(0, 8) : "—"));

const userMenuOptions = computed<DropdownOption[]>(() => [
  { label: "Profile", key: "profile", icon: () => h(NIcon, null, { default: () => h(PersonOutline) }) },
  { type: "divider", key: "d1" },
  { label: "Log out", key: "logout", icon: () => h(NIcon, null, { default: () => h(LogOutOutline) }) }
]);

function onUserMenuSelect(key: string | number): void {
  if (key === "profile") {
    void router.push({ name: "profile" });
  } else if (key === "logout") {
    confirmLogout();
  }
}

function confirmLogout(): void {
  confirmAction({
    title: "Log out",
    content: "Log out from the current session?",
    positiveText: "Log out",
    onConfirm: () => {
      authStore.logout();
      void router.push({ name: "login" });
    }
  });
}

function deploy(): void {
  if (deploying.value) return;
  confirmAction({
    title: "Deploy snapshot",
    content: "Push the latest configuration snapshot to all gateways?",
    positiveText: "Deploy",
    onConfirm: runDeploy
  });
}

async function runDeploy(): Promise<void> {
  try {
    await snapshotStore.deploy();
    message.success("Deploy started");
    void snapshotStore.loadVersion();
  } catch (error) {
    message.error(apiErrorMessage(error, "Unable to start deploy"));
  }
}

// ---- Sidebar resize -------------------------------------------------------

let resizing = false;
let startX = 0;
let startWidth = 0;

function onMouseMove(event: MouseEvent): void {
  if (!resizing) return;
  uiStore.setSidebarWidth(startWidth + (event.clientX - startX));
}

function stopResize(): void {
  if (!resizing) return;
  resizing = false;
  document.body.style.userSelect = "";
  document.body.style.cursor = "";
  document.removeEventListener("mousemove", onMouseMove);
  document.removeEventListener("mouseup", stopResize);
}

function startResize(event: MouseEvent): void {
  event.preventDefault();
  resizing = true;
  startX = event.clientX;
  startWidth = sidebarWidth.value;
  document.body.style.userSelect = "none";
  document.body.style.cursor = "col-resize";
  document.addEventListener("mousemove", onMouseMove);
  document.addEventListener("mouseup", stopResize);
}

function onEscKey(event: KeyboardEvent): void {
  if (event.key === "Escape") {
    uiStore.closeMobileSidebar();
  }
}

watch(
  () => route.fullPath,
  () => uiStore.closeMobileSidebar()
);

onMounted(() => {
  void snapshotStore.loadVersion();
  document.addEventListener("keydown", onEscKey);
});

onBeforeUnmount(() => {
  stopResize();
  document.removeEventListener("keydown", onEscKey);
});
</script>

<template>
  <div class="shell">
    <header class="topbar">
      <div class="topbar__left">
        <NButton
          class="topbar__menu-btn"
          quaternary
          size="small"
          aria-label="Toggle navigation"
          @click="uiStore.openMobileSidebar()"
        >
          <template #icon><NIcon :component="MenuOutline" /></template>
        </NButton>
        <RouterLink to="/" class="topbar__brand">
          <BrandLogo />
        </RouterLink>
        <nav class="topbar__nav">
          <RouterLink
            v-for="link in navLinks"
            :key="link.name"
            :to="{ name: link.name }"
            class="nav-link"
            :class="{ 'nav-link--active': activeNav === link.name }"
          >
            {{ link.label }}
          </RouterLink>
        </nav>
      </div>

      <div class="topbar__right">
        <div class="snapshot-chip" title="Current configuration snapshot version">
          <span class="snapshot-chip__label">snapshot</span>
          <span class="snapshot-chip__value mono">{{ shortVersion }}</span>
        </div>
        <NButton
          size="small"
          tertiary
          :loading="deploying"
          aria-label="Deploy"
          @click="deploy"
        >
          <template #icon>
            <NIcon :component="deploying ? SyncOutline : RocketOutline" />
          </template>
          <span class="topbar__deploy-label">Deploy</span>
        </NButton>
        <NDropdown
          trigger="click"
          :options="userMenuOptions"
          placement="bottom-end"
          @select="onUserMenuSelect"
        >
          <button class="user-trigger" type="button">
            <span class="user-trigger__avatar">{{ profileInitial }}</span>
            <span class="user-trigger__name">{{ profileName }}</span>
            <NTag
              size="tiny"
              :type="authStore.isAdmin ? 'success' : 'default'"
              :bordered="false"
            >
              {{ authStore.isAdmin ? "admin" : "user" }}
            </NTag>
          </button>
        </NDropdown>
      </div>
    </header>

    <div class="body">
      <aside
        class="sider"
        :class="{ 'sider--mobile-open': mobileSidebarOpen }"
        :style="{ width: `${sidebarWidth}px` }"
      >
        <nav class="mobile-nav">
          <RouterLink
            v-for="link in navLinks"
            :key="link.name"
            :to="{ name: link.name }"
            class="mobile-nav__link"
            :class="{ 'mobile-nav__link--active': activeNav === link.name }"
            @click="uiStore.closeMobileSidebar()"
          >
            {{ link.label }}
          </RouterLink>
        </nav>
        <div class="sider__apps">
          <AppNavSidebar @navigate="uiStore.closeMobileSidebar()" />
        </div>
      </aside>
      <button
        class="resize-handle"
        type="button"
        aria-label="Resize sidebar"
        :aria-valuenow="sidebarWidth"
        :aria-valuemin="SIDEBAR_MIN_WIDTH"
        :aria-valuemax="SIDEBAR_MAX_WIDTH"
        @mousedown="startResize"
      />
      <button
        v-if="mobileSidebarOpen"
        class="scrim"
        type="button"
        aria-label="Close navigation"
        @click="uiStore.closeMobileSidebar()"
      />

      <main class="content">
        <RouterView v-slot="{ Component, route: currentRoute }">
          <Transition name="page" mode="out-in">
            <component :is="Component" :key="currentRoute.fullPath" />
          </Transition>
        </RouterView>
      </main>
    </div>

    <AppFormDrawer />
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

/* ---- Topbar -------------------------------------------------------------- */

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  height: 56px;
  padding: 0 16px;
  flex-shrink: 0;
  background: var(--c-bg-soft);
  border-bottom: 1px solid var(--c-border);
}

.topbar__left {
  display: flex;
  align-items: center;
  gap: 18px;
  min-width: 0;
}

.topbar__menu-btn {
  display: none;
}

.topbar__brand {
  display: flex;
  align-items: center;
}

.topbar__nav {
  display: flex;
  align-items: center;
  gap: 2px;
}

.nav-link {
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 13.5px;
  font-weight: 500;
  color: var(--c-text-2);
  transition:
    background-color 0.14s ease,
    color 0.14s ease;
}

.nav-link:hover {
  color: var(--c-text);
  background: rgba(255, 255, 255, 0.04);
}

.nav-link--active {
  color: var(--c-text);
  background: var(--c-primary-soft);
}

.topbar__right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.snapshot-chip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  height: 30px;
  padding: 0 11px;
  border-radius: 8px;
  border: 1px solid var(--c-border);
  background: var(--c-surface);
}

.snapshot-chip__label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--c-text-3);
}

.snapshot-chip__value {
  font-size: 12.5px;
  color: var(--c-text-2);
}

.user-trigger {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 34px;
  padding: 0 10px 0 4px;
  border: 1px solid var(--c-border);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-text);
  cursor: pointer;
  transition: border-color 0.14s ease;
}

.user-trigger:hover {
  border-color: var(--c-border-strong);
}

.user-trigger__avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border-radius: 999px;
  background: linear-gradient(135deg, #63e2b7, #4bb592);
  color: #0b1f18;
  font-size: 12px;
  font-weight: 700;
}

.user-trigger__name {
  font-size: 13px;
  font-weight: 500;
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ---- Body / sider / content --------------------------------------------- */

.body {
  position: relative;
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
}

.sider {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  height: 100%;
  background: var(--c-bg-soft);
  border-right: 1px solid var(--c-border);
  overflow: hidden;
}

.sider__apps {
  flex: 1 1 auto;
  min-height: 0;
}

/* Primary navigation, surfaced inside the drawer on mobile only. */
.mobile-nav {
  display: none;
  flex-direction: column;
  gap: 2px;
  padding: 12px 12px 10px;
  border-bottom: 1px solid var(--c-border);
}

.mobile-nav__link {
  padding: 9px 12px;
  border-radius: 9px;
  font-size: 14px;
  font-weight: 500;
  color: var(--c-text-2);
  transition:
    background-color 0.14s ease,
    color 0.14s ease;
}

.mobile-nav__link:hover {
  color: var(--c-text);
  background: rgba(255, 255, 255, 0.04);
}

.mobile-nav__link--active {
  color: var(--c-text);
  background: var(--c-primary-soft);
}

.resize-handle {
  width: 6px;
  margin-left: -6px;
  border: none;
  padding: 0;
  background: transparent;
  cursor: col-resize;
  z-index: 5;
  transition: background-color 0.14s ease;
}

.resize-handle:hover,
.resize-handle:focus-visible {
  background: var(--c-primary-soft);
  outline: none;
}

.content {
  flex: 1 1 auto;
  min-width: 0;
  height: 100%;
  overflow: auto;
}

.scrim {
  display: none;
}

/* ---- Responsive ---------------------------------------------------------- */

@media (max-width: 900px) {
  .topbar__menu-btn {
    display: inline-flex;
  }

  .topbar__nav {
    display: none;
  }

  .mobile-nav {
    display: flex;
  }

  .snapshot-chip {
    display: none;
  }

  .sider {
    position: absolute;
    z-index: 30;
    inset: 0 auto 0 0;
    transform: translateX(-100%);
    transition: transform 0.2s ease;
    box-shadow: var(--shadow-lg);
  }

  .sider--mobile-open {
    transform: translateX(0);
  }

  .resize-handle {
    display: none;
  }

  .scrim {
    display: block;
    position: absolute;
    inset: 0;
    z-index: 20;
    border: none;
    background: rgba(0, 0, 0, 0.5);
    cursor: pointer;
  }
}

@media (max-width: 560px) {
  .topbar {
    padding: 0 12px;
    gap: 10px;
  }

  .topbar__left {
    gap: 10px;
  }

  .topbar__right {
    gap: 8px;
  }

  .user-trigger__name,
  .topbar__deploy-label {
    display: none;
  }

  /* Logo-only brand to save horizontal space */
  .topbar__brand :deep(.brand__text) {
    display: none;
  }
}
</style>
