import { createRouter, createWebHistory } from "vue-router";
import LoginPage from "../pages/LoginPage.vue";
import AppShell from "../components/AppShell.vue";
import DashboardPage from "../pages/DashboardPage.vue";
import AppFormPage from "../pages/AppFormPage.vue";
import AppDetailsPage from "../pages/AppDetailsPage.vue";
import EndpointFormPage from "../pages/EndpointFormPage.vue";
import RootFormPage from "../pages/RootFormPage.vue";
import ProfilePage from "../pages/ProfilePage.vue";
import UsersPage from "../pages/UsersPage.vue";
import UsersFormPage from "../pages/UsersFormPage.vue";
import { useAuthStore } from "../stores/auth";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/login",
      name: "login",
      component: LoginPage,
      meta: { public: true }
    },
    {
      path: "/",
      component: AppShell,
      meta: { requiresAuth: true },
      children: [
        {
          path: "",
          name: "dashboard",
          component: DashboardPage
        },
        {
          path: "apps/new",
          name: "app-create",
          component: AppFormPage
        },
        {
          path: "apps/:id",
          name: "app-details",
          component: AppDetailsPage
        },
        {
          path: "apps/:id/edit",
          name: "app-edit",
          component: AppFormPage
        },
        {
          path: "apps/:appId/endpoints/new",
          name: "endpoint-create",
          component: EndpointFormPage
        },
        {
          path: "endpoints/:id/edit",
          name: "endpoint-edit",
          component: EndpointFormPage
        },
        {
          path: "root/edit",
          name: "root-edit",
          component: RootFormPage
        },
        {
          path: "profile",
          name: "profile",
          component: ProfilePage
        },
        {
          path: "users",
          name: "users",
          component: UsersPage,
          meta: { requiresAdmin: true }
        },
        {
          path: "users/new",
          name: "users-create",
          component: UsersFormPage,
          meta: { requiresAdmin: true }
        },
        {
          path: "users/:id/edit",
          name: "users-edit",
          component: UsersFormPage,
          meta: { requiresAdmin: true }
        }
      ]
    }
  ]
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

  if (to.meta.public as boolean | undefined) {
    if (authStore.isAuthenticated) {
      return { name: "dashboard" };
    }
    return true;
  }

  if ((to.meta.requiresAuth as boolean | undefined) && !authStore.isAuthenticated) {
    return {
      name: "login",
      query: { redirect: to.fullPath }
    };
  }

  if ((to.meta.requiresAdmin as boolean | undefined) && !authStore.profile?.is_admin) {
    return { name: "dashboard" };
  }
  return true;
});
