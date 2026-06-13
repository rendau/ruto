<script setup lang="ts">
import { useRouter } from "vue-router";
import { NButton, NResult } from "naive-ui";

const router = useRouter();
</script>

<template>
  <div class="not-found">
    <NResult status="404" title="Page not found" description="This page does not exist.">
      <template #footer>
        <NButton type="primary" @click="router.push('/')">Back to dashboard</NButton>
      </template>
    </NResult>
  </div>
</template>

<style scoped>
.not-found {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  padding: 24px;
}
</style>
