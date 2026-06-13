import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useAuthStore } from "@/stores/auth";

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
    component: () => import("@/views/LoginView.vue"),
    meta: { title: "Sign in", public: true }
  },
  {
    path: "/",
    component: () => import("@/layouts/DefaultLayout.vue"),
    meta: { requiresAuth: true },
    children: [
      {
        path: "",
        name: "dashboard",
        component: () => import("@/views/DashboardView.vue"),
        meta: { title: "Dashboard" }
      },
      {
        path: "apps",
        name: "apps",
        component: () => import("@/views/AppWorkspaceView.vue"),
        meta: { title: "Applications" }
      },
      {
        path: "apps/:id",
        name: "app-workspace",
        component: () => import("@/views/AppWorkspaceView.vue"),
        meta: { title: "Application" }
      },
      {
        path: "root",
        name: "root-config",
        component: () => import("@/views/RootConfigView.vue"),
        meta: { title: "Root configuration" }
      },
      {
        path: "users",
        name: "users",
        component: () => import("@/views/UsrListView.vue"),
        meta: { title: "Users", requiresAdmin: true }
      },
      {
        path: "gateways",
        name: "gateways",
        component: () => import("@/views/GatewayListView.vue"),
        meta: { title: "Gateways" }
      },
      {
        path: "profile",
        name: "profile",
        component: () => import("@/views/ProfileView.vue"),
        meta: { title: "Profile" }
      }
    ]
  },
  {
    path: "/:pathMatch(.*)*",
    name: "not-found",
    component: () => import("@/views/NotFoundView.vue"),
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
