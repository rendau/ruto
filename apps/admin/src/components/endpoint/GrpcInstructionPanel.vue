<script setup lang="ts">
import { computed } from "vue";
import { NAlert, NIcon, NModal, NTag } from "naive-ui";
import { CopyOutline } from "@vicons/ionicons5";
import { useRootStore } from "@/stores/root";
import { useClipboard } from "@/composables/useClipboard";
import JsonBlock from "@/components/common/JsonBlock.vue";
import type { AppMain } from "@/api/types";

const props = defineProps<{ show: boolean; app: AppMain }>();
const emit = defineEmits<{ "update:show": [value: boolean] }>();

const rootStore = useRootStore();
const { copy } = useClipboard();

const connection = computed(() => {
  const baseUrl = rootStore.baseUrl;
  if (!baseUrl) return null;
  try {
    const url = new URL(baseUrl);
    const isSecure = url.protocol === "https:";
    return {
      address: `${url.hostname}:${url.port || (isSecure ? "443" : "80")}`,
      isSecure,
      headerName: "x-ruto-app-name",
      headerValue: props.app.name || props.app.id
    };
  } catch {
    return null;
  }
});

const copyItems = computed(() => {
  if (!connection.value) return [];
  return [
    {
      key: "address",
      label: "Gateway gRPC address",
      value: connection.value.address,
      badge: connection.value.isSecure ? "Secure" : "Plaintext",
      hint: "If gRPC is exposed on a separate port, replace only the port."
    },
    {
      key: "header",
      label: "Metadata header",
      value: `${connection.value.headerName}: ${connection.value.headerValue}`,
      hint: "Required on every call — the gateway routes by application name."
    }
  ];
});

const grpcurlExample = computed(() => {
  if (!connection.value) return "";
  const { address, isSecure, headerName, headerValue } = connection.value;
  const plaintext = isSecure ? "" : " \\\n  -plaintext";
  return `grpcurl${plaintext} \\
  -H '${headerName}: ${headerValue}' \\
  -d '{"id":"123"}' \\
  ${address} \\
  package.Service/Method`;
});
</script>

<template>
  <NModal
    :show="show"
    preset="card"
    title="gRPC connection"
    class="grpc-modal"
    :bordered="false"
    @update:show="(value: boolean) => emit('update:show', value)"
  >
    <div v-if="connection" class="grpc">
      <div class="grpc__tiles">
        <button
          v-for="item in copyItems"
          :key="item.key"
          type="button"
          class="grpc__tile"
          @click="copy(item.value, `${item.label} copied`)"
        >
          <span class="grpc__tile-label">{{ item.label }}</span>
          <span class="grpc__tile-value-row">
            <code class="grpc__tile-value">{{ item.value }}</code>
            <NTag v-if="item.badge" size="tiny" :bordered="false" type="info">{{ item.badge }}</NTag>
          </span>
          <span class="grpc__tile-hint muted">{{ item.hint }}</span>
          <span class="grpc__tile-copy">
            <NIcon :component="CopyOutline" :size="13" /> Click to copy
          </span>
        </button>
      </div>

      <NAlert type="info" :bordered="false" :show-icon="false" class="grpc__note">
        The gateway matches calls by <code>x-ruto-app-name</code> and the full method path
        <code>/package.Service/Method</code>.
      </NAlert>

      <div class="grpc__example">
        <span class="section-label">grpcurl example</span>
        <JsonBlock :content="grpcurlExample" max-height="240px" />
      </div>
    </div>

    <NAlert v-else type="warning" :bordered="false">
      Base URL is not configured. Set it in Root configuration first, then reopen this guide.
    </NAlert>
  </NModal>
</template>

<style scoped>
:global(.grpc-modal) {
  width: min(760px, calc(100vw - 48px));
}

.grpc {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.grpc__tiles {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.grpc__tile {
  display: flex;
  flex-direction: column;
  gap: 7px;
  padding: 12px;
  border: 1px solid var(--c-border);
  border-radius: 10px;
  background: var(--c-surface-2);
  text-align: left;
  cursor: pointer;
  transition:
    border-color 0.14s ease,
    transform 0.14s ease;
}

.grpc__tile:hover {
  border-color: var(--c-border-strong);
  transform: translateY(-1px);
}

.grpc__tile-label {
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--c-text-3);
}

.grpc__tile-value-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.grpc__tile-value {
  font-family: var(--font-mono);
  font-size: 13.5px;
  font-weight: 600;
  color: var(--c-text);
  overflow-wrap: anywhere;
}

.grpc__tile-hint {
  font-size: 12px;
  line-height: 1.4;
}

.grpc__tile-copy {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11.5px;
  color: var(--c-text-3);
}

.grpc__note code {
  font-family: var(--font-mono);
  color: var(--c-text);
}

.grpc__example {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

@media (max-width: 560px) {
  .grpc__tiles {
    grid-template-columns: 1fr;
  }
}
</style>
