<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(defineProps<{ method: string; grpc?: boolean }>(), { grpc: false });

const label = computed(() => (props.grpc ? "gRPC" : (props.method || "").toUpperCase() || "—"));

const kind = computed(() => {
  if (props.grpc) return "grpc";
  const method = (props.method || "").trim().toUpperCase();
  switch (method) {
    case "GET":
      return "get";
    case "POST":
      return "post";
    case "PUT":
      return "put";
    case "PATCH":
      return "patch";
    case "DELETE":
      return "delete";
    default:
      return "default";
  }
});
</script>

<template>
  <span class="method-badge mono" :class="`method-badge--${kind}`">{{ label }}</span>
</template>

<style scoped>
.method-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 52px;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.03em;
  line-height: 1.5;
}

.method-badge--get {
  background: rgba(99, 226, 183, 0.16);
  color: #63e2b7;
}

.method-badge--post {
  background: rgba(112, 192, 232, 0.16);
  color: #70c0e8;
}

.method-badge--put {
  background: rgba(242, 201, 125, 0.16);
  color: #f2c97d;
}

.method-badge--patch {
  background: rgba(201, 164, 244, 0.16);
  color: #c9a4f4;
}

.method-badge--delete {
  background: rgba(232, 128, 128, 0.16);
  color: #e88080;
}

.method-badge--grpc {
  background: rgba(122, 214, 201, 0.16);
  color: #7ad6c9;
}

.method-badge--default {
  background: rgba(255, 255, 255, 0.08);
  color: var(--c-text-2);
}
</style>
