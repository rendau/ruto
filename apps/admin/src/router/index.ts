import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import DefaultLayout from "@/layouts/DefaultLayout.vue";
import AppWorkspaceView from "@/views/AppWorkspaceView.vue";
import ChangeRequestListView from "@/views/ChangeRequestListView.vue";
import DashboardView from "@/views/DashboardView.vue";
import GatewayListView from "@/views/GatewayListView.vue";
import LoginView from "@/views/LoginView.vue";
import NotFoundView from "@/views/NotFoundView.vue";
import OAuthAuthorizeView from "@/views/OAuthAuthorizeView.vue";
import ProfileView from "@/views/ProfileView.vue";
import RootConfigView from "@/views/RootConfigView.vue";
import UsrListView from "@/views/UsrListView.vue";

declare module "vue-router" {
  interface RouteMeta {
    title?: string;
    public?: boolean;
    requiresAuth?: boolean;
    requiresAdmin?: boolean;
  }
}

const APP_NAME = "Ruto Admin";

const routes: RouteRecordRaw[] = [
  {
    path: "/login",
    name: "login",
    component: LoginView,
    meta: { title: "Sign in", public: true }
  },
  {
    // consent page of the MCP server's OAuth flow; the MCP client opens it
    path: "/oauth/authorize",
    name: "oauth-authorize",
    component: OAuthAuthorizeView,
    meta: { title: "Authorize access", requiresAuth: true }
  },
  {
    path: "/",
    component: DefaultLayout,
    meta: { requiresAuth: true },
    children: [
      {
        path: "",
        name: "dashboard",
        component: DashboardView,
        meta: { title: "Dashboard" }
      },
      {
        path: "apps",
        name: "apps",
        component: AppWorkspaceView,
        meta: { title: "Applications" }
      },
      {
        path: "apps/:id",
        name: "app-workspace",
        component: AppWorkspaceView,
        meta: { title: "Application" }
      },
      {
        path: "root",
        name: "root-config",
        component: RootConfigView,
        meta: { title: "Root configuration" }
      },
      {
        path: "users",
        name: "users",
        component: UsrListView,
        meta: { title: "Users", requiresAdmin: true }
      },
      {
        path: "change-requests",
        name: "change-requests",
        component: ChangeRequestListView,
        meta: { title: "Change requests", requiresAdmin: true }
      },
      {
        path: "gateways",
        name: "gateways",
        component: GatewayListView,
        meta: { title: "Gateways" }
      },
      {
        path: "profile",
        name: "profile",
        component: ProfileView,
        meta: { title: "Profile" }
      }
    ]
  },
  {
    path: "/:pathMatch(.*)*",
    name: "not-found",
    component: NotFoundView,
    meta: { title: "Not found" }
  }
];

export const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior() {
    return { top: 0 };
  }
});

router.beforeEach(async (to) => {
  const authStore = useAuthStore();

  if (!authStore.initialized) {
    try {
      await authStore.initialize();
    } catch {
      authStore.logout();
    }
  }

  if (to.meta.public) {
    return authStore.isAuthenticated ? { name: "dashboard" } : true;
  }

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    return { name: "login", query: { redirect: to.fullPath } };
  }

  if (to.meta.requiresAdmin && !authStore.isAdmin) {
    return { name: "dashboard" };
  }

  return true;
});

router.afterEach((to) => {
  const title = to.meta.title;
  document.title = title ? `${title} · ${APP_NAME}` : APP_NAME;
});
