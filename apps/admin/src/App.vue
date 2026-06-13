<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { darkTheme } from "naive-ui";
import { themeOverrides } from "@/theme";
import { useAuthStore } from "@/stores/auth";
import NaiveBridge from "@/components/common/NaiveBridge.vue";

const router = useRouter();
const authStore = useAuthStore();

function onAuthRequired(): void {
  authStore.logout();
  void router.push({ name: "login" });
}

onMounted(() => {
  window.addEventListener("auth:required", onAuthRequired);
});

onUnmounted(() => {
  window.removeEventListener("auth:required", onAuthRequired);
});
</script>

<template>
  <n-config-provider :theme="darkTheme" :theme-overrides="themeOverrides">
    <n-global-style />
    <n-loading-bar-provider>
      <n-message-provider placement="top-right" :max="4">
        <n-dialog-provider>
          <n-notification-provider>
            <NaiveBridge>
              <router-view />
            </NaiveBridge>
          </n-notification-provider>
        </n-dialog-provider>
      </n-message-provider>
    </n-loading-bar-provider>
  </n-config-provider>
</template>
