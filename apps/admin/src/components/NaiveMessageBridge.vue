<script setup lang="ts">
import { onMounted } from "vue";
import { useMessage } from "naive-ui";
import { setMessageApi } from "../lib/notify";

const message = useMessage();

onMounted(() => {
  setMessageApi(message);
});
</script>

<template></template>
