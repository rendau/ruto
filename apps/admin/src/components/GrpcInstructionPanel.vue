<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { notifyError, notifySuccess } from "../lib/notify";
import type { AppMain, RootMain } from "../types/api";

const props = defineProps<{
  app: AppMain;
  root: RootMain | null;
  open: boolean;
}>();

const emit = defineEmits<{
  (e: "close"): void;
}>();

type CopyItem = {
  key: string;
  label: string;
  value: string;
  hint?: string;
  badge?: string;
};

const copiedKey = ref("");
let copiedTimer: number | undefined;

const connectionInfo = computed(() => {
  if (!props.root?.base_url) return null;

  try {
    const url = new URL(props.root.base_url);
    const isSecure = url.protocol === "https:";
    const host = url.hostname;
    const port = url.port || (isSecure ? "443" : "80");
    const address = `${host}:${port}`;

    return {
      host,
      port,
      isSecure,
      address,
      headerName: "x-ruto-app-name",
      headerValue: props.app.name
    };
  } catch {
    return null;
  }
});

const copyItems = computed<CopyItem[]>(() => {
  if (!connectionInfo.value) return [];

  return [
    {
      key: "gateway-address",
      label: "Gateway gRPC address",
      value: connectionInfo.value.address,
      badge: connectionInfo.value.isSecure ? "Secure" : "Plaintext",
      hint: "Use the gateway gRPC listener. If gRPC is exposed on a separate port, replace only the port."
    },
    {
      key: "app-header",
      label: "Metadata header",
      value: `${connectionInfo.value.headerName}: ${connectionInfo.value.headerValue}`,
      hint: "Required for every call, because gateway routes by application name."
    }
  ];
});

const grpcurlCallExample = computed(() => {
  if (!connectionInfo.value) return "";
  const { address, isSecure, headerName, headerValue } = connectionInfo.value;
  const plaintextFlag = isSecure ? "" : " \\\n  -plaintext";

  return `grpcurl${plaintextFlag} \\
  -H '${headerName}: ${headerValue}' \\
  -d '{"id":"123"}' \\
  ${address} \\
  package.Service/Method`;
});

function close(): void {
  if (!props.open) {
    return;
  }
  emit("close");
}

function onKeydown(event: KeyboardEvent): void {
  if (props.open && event.key === "Escape") {
    close();
  }
}

async function copyText(key: string, label: string, value: string): Promise<void> {
  if (!value) {
    return;
  }

  try {
    await navigator.clipboard.writeText(value);
    copiedKey.value = key;
    notifySuccess(`${label} copied`);
    if (copiedTimer) {
      window.clearTimeout(copiedTimer);
    }
    copiedTimer = window.setTimeout(() => {
      copiedKey.value = "";
    }, 1600);
  } catch {
    notifyError(`Unable to copy ${label.toLowerCase()}`);
  }
}

onMounted(() => {
  window.addEventListener("keydown", onKeydown);
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", onKeydown);
  if (copiedTimer) {
    window.clearTimeout(copiedTimer);
  }
});
</script>

<template>
  <n-modal
    :show="open"
    preset="card"
    class="grpc-modal-card"
    :bordered="false"
    :mask-closable="true"
    content-style="display: flex; min-height: 0; height: 100%; overflow: hidden;"
    @update:show="(value: boolean) => { if (!value) close(); }"
  >
    <template #header>
      <div class="grpc-modal-head">
        <div>
          <p class="grpc-modal-kicker">gRPC connection</p>
          <h3 id="grpc-modal-title">Connect to {{ app.name }}</h3>
          <p id="grpc-modal-description" class="grpc-modal-subtitle">
            Copy the gateway address, required metadata, and ready-to-run examples.
          </p>
        </div>
      </div>
    </template>

    <div v-if="connectionInfo" class="grpc-modal-body">
      <div class="grpc-copy-grid" aria-label="Connection values">
        <button
          v-for="item in copyItems"
          :key="item.key"
          class="grpc-copy-tile"
          type="button"
          :title="`Copy ${item.label}`"
          :aria-label="`Copy ${item.label}`"
          @click="copyText(item.key, item.label, item.value)"
        >
          <span class="grpc-copy-label">{{ item.label }}</span>
          <span class="grpc-copy-value-row">
            <code class="grpc-copy-value">{{ item.value }}</code>
            <span v-if="item.badge" class="grpc-copy-badge">{{ item.badge }}</span>
          </span>
          <span class="grpc-copy-hint">{{ item.hint }}</span>
          <span class="grpc-copy-state">{{ copiedKey === item.key ? "Copied" : "Click to copy" }}</span>
        </button>
      </div>

      <div class="grpc-note">
        <strong>Route lookup:</strong>
        gateway matches calls by <code>x-ruto-app-name</code> and full method path like
        <code>/package.Service/Method</code>.
      </div>

      <section class="grpc-code-section" aria-labelledby="grpc-call-title">
        <div class="grpc-code-head">
          <div>
            <h4 id="grpc-call-title">Call example</h4>
            <p>Replace package, service, method, and payload with your endpoint contract.</p>
          </div>
          <n-button secondary @click="copyText('grpcurl-call', 'grpcurl call example', grpcurlCallExample)">
            {{ copiedKey === "grpcurl-call" ? "Copied" : "Copy" }}
          </n-button>
        </div>
        <button class="grpc-code-copy" type="button" @click="copyText('grpcurl-call', 'grpcurl call example', grpcurlCallExample)">
          <pre><code>{{ grpcurlCallExample }}</code></pre>
        </button>
      </section>
    </div>

    <div v-else class="grpc-modal-empty">
      <n-alert class="form-alert" type="error" :show-icon="false">Base URL is not configured in Root Settings.</n-alert>
      <p class="muted">Configure Root Base URL first, then reopen this connection guide.</p>
    </div>
  </n-modal>
</template>

<style scoped>
:global(.grpc-modal-card) {
  width: min(960px, calc(100vw - 48px));
  height: min(840px, calc(100dvh - 48px));
  max-height: calc(100dvh - 48px);
  margin: auto;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

:global(.grpc-modal-card > .n-card-header) {
  flex: 0 0 auto;
  min-height: 0;
  padding: 0;
}

:global(.grpc-modal-card .n-card__content) {
  flex: 1 1 auto;
  min-height: 0;
  height: 100%;
  display: flex;
  overflow: hidden;
  max-height: none;
  padding: 0 14px 14px !important;
}

.grpc-modal-head {
  position: sticky;
  top: 0;
  z-index: 1;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 22px 16px;
  border-bottom: 1px solid #344d73;
  background: #172338;
}

.grpc-modal-kicker {
  margin: 0 0 5px;
  color: #91b4df;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.grpc-modal-head h3 {
  margin: 0;
  color: #f3f7ff;
  font-size: 24px;
  line-height: 1.2;
  overflow-wrap: anywhere;
}

.grpc-modal-subtitle {
  margin: 7px 0 0;
  color: #9eb2cf;
  font-size: 14px;
}

.grpc-modal-close {
  flex: 0 0 auto;
}

.grpc-modal-body,
.grpc-modal-empty {
  flex: 1 1 0;
  min-height: 0;
  height: 100%;
  max-height: none;
  display: grid;
  gap: 16px;
  overflow-y: auto;
  overscroll-behavior: contain;
  -webkit-overflow-scrolling: touch;
  padding: 18px 22px 22px;
}

.grpc-copy-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.grpc-copy-tile {
  min-width: 0;
  display: grid;
  gap: 7px;
  padding: 12px;
  border: 1px solid #3f597c;
  border-radius: 8px;
  background: #1e2d44;
  color: inherit;
  text-align: left;
  cursor: pointer;
  transition: background-color 0.16s ease, border-color 0.16s ease, transform 0.16s ease;
}

.grpc-copy-tile:hover,
.grpc-copy-tile:focus-visible {
  border-color: #6ea6dc;
  background: #243954;
  transform: translateY(-1px);
  outline: none;
}

.grpc-copy-label {
  color: #9cb1cd;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}

.grpc-copy-value {
  color: #f4f8ff;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 14px;
  font-weight: 700;
  overflow-wrap: anywhere;
}

.grpc-copy-value-row {
  min-width: 0;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.grpc-copy-badge {
  min-height: 22px;
  display: inline-flex;
  align-items: center;
  padding: 3px 8px;
  border: 1px solid #4d6487;
  border-radius: 999px;
  background: #263a56;
  color: #c8e7ff;
  font-size: 12px;
  font-weight: 700;
  line-height: 1;
}

.grpc-copy-hint {
  color: #a8bad4;
  font-size: 12px;
  line-height: 1.35;
}

.grpc-copy-state {
  justify-self: start;
  min-height: 22px;
  display: inline-flex;
  align-items: center;
  padding: 3px 8px;
  border: 1px solid #4d6487;
  border-radius: 999px;
  background: #263a56;
  color: #c8e7ff;
  font-size: 12px;
  font-weight: 700;
}

.grpc-note {
  padding: 12px;
  border: 1px solid #3f597c;
  border-radius: 8px;
  background: #20314a;
  color: #cbd8ec;
  font-size: 14px;
}

.grpc-note code {
  color: #f4f8ff;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}

.grpc-code-section {
  display: grid;
  gap: 10px;
}

.grpc-code-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.grpc-code-head h4 {
  margin: 0 0 3px;
  color: #eef5ff;
  font-size: 16px;
}

.grpc-code-head p {
  margin: 0;
  color: #9eb2cf;
  font-size: 13px;
}

.grpc-code-copy {
  width: 100%;
  min-width: 0;
  padding: 0;
  border: 1px solid #334b70;
  border-radius: 8px;
  background: #101827;
  color: #d4e4fa;
  text-align: left;
  cursor: pointer;
  overflow: hidden;
}

.grpc-code-copy:hover,
.grpc-code-copy:focus-visible {
  border-color: #6ea6dc;
  outline: none;
}

.grpc-code-copy pre {
  margin: 0;
  padding: 14px;
  overflow-x: auto;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 13px;
  line-height: 1.55;
}

.grpc-code-copy code {
  font-family: inherit;
}

@media (max-width: 720px) {
  :global(.grpc-modal-card) {
    width: calc(100vw - 4px);
    height: calc(100dvh - 4px);
    max-height: calc(100dvh - 4px);
    margin: 2px auto;
  }

  .grpc-modal-head {
    padding: 8px;
  }

  .grpc-modal-body,
  .grpc-modal-empty {
    max-height: none;
  }

  .grpc-modal-head h3 {
    font-size: 20px;
  }

  .grpc-modal-body,
  .grpc-modal-empty {
    padding: 0 4px 8px;
  }

  :global(.grpc-modal-card .n-card-content),
  :global(.grpc-modal-card .n-card__content) {
    padding: 0 4px 4px !important;
  }

  .grpc-copy-grid {
    grid-template-columns: 1fr;
  }

  .grpc-code-head {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
