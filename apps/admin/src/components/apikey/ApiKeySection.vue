<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import {
  NAlert,
  NButton,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NSpin,
  useMessage,
  type FormInst,
  type FormRules
} from "naive-ui";
import { AddOutline, CopyOutline, KeyOutline, TrashOutline } from "@vicons/ionicons5";
import { createApiKey, deleteApiKey, listApiKeys } from "@/api/apiKey";
import { apiErrorMessage } from "@/api/http";
import { useConfirm } from "@/composables/useConfirm";
import { useClipboard } from "@/composables/useClipboard";
import { formatUnixAge, formatUnixDateTime } from "@/lib/datetime";
import SectionCard from "@/components/common/SectionCard.vue";
import type { ApiKeyMain } from "@/api/types";

const message = useMessage();
const { confirmDelete } = useConfirm();
const { copy } = useClipboard();

const keys = ref<ApiKeyMain[]>([]);
const loading = ref(false);

const showCreate = ref(false);
const creating = ref(false);
const formRef = ref<FormInst | null>(null);
const model = reactive({ name: "" });
const rules: FormRules = {
  name: [{ required: true, message: "Name is required", trigger: ["blur", "input"] }]
};

// The plain key is returned only once by the backend; it lives here until the
// admin closes the reveal dialog.
const createdKey = ref("");
const createdName = ref("");

async function fetchKeys(): Promise<void> {
  loading.value = true;
  try {
    const rep = await listApiKeys();
    keys.value = rep.results;
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to load API keys"));
  } finally {
    loading.value = false;
  }
}

function openCreate(): void {
  model.name = "";
  showCreate.value = true;
}

async function submitCreate(): Promise<void> {
  try {
    await formRef.value?.validate();
  } catch {
    return;
  }
  creating.value = true;
  try {
    const name = model.name.trim();
    const rep = await createApiKey(name);
    showCreate.value = false;
    createdName.value = name;
    createdKey.value = rep.key;
    await fetchKeys();
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to create API key"));
  } finally {
    creating.value = false;
  }
}

function closeCreated(): void {
  createdKey.value = "";
  createdName.value = "";
}

function remove(key: ApiKeyMain): void {
  confirmDelete({
    title: "Delete API key",
    content: `Delete key "${key.name}"? Agents using it lose access immediately.`,
    onConfirm: async () => {
      try {
        await deleteApiKey(key.id);
        message.success("API key deleted");
        await fetchKeys();
      } catch (error) {
        message.error(apiErrorMessage(error, "Failed to delete API key"));
      }
    }
  });
}

onMounted(fetchKeys);
</script>

<template>
  <SectionCard title="API keys" description="Access for AI agents to the built-in MCP server">
    <template #extra>
      <NButton size="small" type="primary" @click="openCreate">
        <template #icon><NIcon :component="AddOutline" /></template>
        Create key
      </NButton>
    </template>

    <NAlert type="info" :bordered="false" :show-icon="false" class="keys__hint">
      An AI agent connects to the MCP server with <code>Authorization: Bearer &lt;key&gt;</code>.
      The key works only on the MCP endpoint, never on the admin API. Agents can read the
      configuration and propose changes; every change waits for an administrator under
      <RouterLink :to="{ name: 'change-requests' }" class="keys__link">Change requests</RouterLink>.
    </NAlert>

    <NSpin :show="loading">
      <div v-if="keys.length" class="keys">
        <div v-for="key in keys" :key="key.id" class="key-row">
          <NIcon :component="KeyOutline" class="key-row__icon" />
          <div class="key-row__main">
            <div class="key-row__head">
              <span class="key-row__name">{{ key.name }}</span>
              <code class="key-row__prefix">{{ key.key_prefix }}…</code>
            </div>
            <div class="key-row__meta muted">
              <span>Created {{ formatUnixDateTime(key.created_at_unix, "—") }}</span>
              <span :title="formatUnixDateTime(key.last_used_at_unix, '')">
                Last used {{ formatUnixAge(key.last_used_at_unix, "never") }}
              </span>
            </div>
          </div>
          <NButton
            class="danger-icon-button"
            quaternary
            circle
            size="small"
            type="error"
            title="Delete"
            @click="remove(key)"
          >
            <template #icon><NIcon :component="TrashOutline" /></template>
          </NButton>
        </div>
      </div>
      <p v-else-if="!loading" class="muted keys__empty">No API keys yet.</p>
    </NSpin>

    <NModal
      :show="showCreate"
      preset="card"
      title="New API key"
      class="api-key-modal"
      :bordered="false"
      @update:show="(value: boolean) => (showCreate = value)"
    >
      <NForm
        ref="formRef"
        :model="model"
        :rules="rules"
        :disabled="creating"
        label-placement="top"
        @submit.prevent="submitCreate"
      >
        <NFormItem label="Name" path="name">
          <NInput
            v-model:value="model.name"
            placeholder="e.g. claude, laptop"
            :input-props="{ autocomplete: 'off' }"
            @keydown.enter.prevent="submitCreate"
          />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="form-actions">
          <NButton :disabled="creating" @click="showCreate = false">Cancel</NButton>
          <NButton type="primary" :loading="creating" @click="submitCreate">Create</NButton>
        </div>
      </template>
    </NModal>

    <NModal
      :show="Boolean(createdKey)"
      preset="card"
      title="API key created"
      class="api-key-modal"
      :bordered="false"
      :mask-closable="false"
      @update:show="(value: boolean) => !value && closeCreated()"
    >
      <div class="created">
        <NAlert type="warning" :bordered="false">
          Copy the key now — it will not be shown again.
        </NAlert>
        <div>
          <span class="section-label">{{ createdName }}</span>
          <code class="created__key">{{ createdKey }}</code>
        </div>
      </div>
      <template #footer>
        <div class="form-actions">
          <NButton @click="closeCreated">Done</NButton>
          <NButton type="primary" @click="copy(createdKey, 'API key copied')">
            <template #icon><NIcon :component="CopyOutline" /></template>
            Copy key
          </NButton>
        </div>
      </template>
    </NModal>
  </SectionCard>
</template>

<style scoped>
:global(.api-key-modal) {
  width: min(480px, calc(100vw - 32px));
}

.keys__hint {
  margin-bottom: 14px;
  font-size: 13px;
}

.keys__hint code {
  font-family: var(--font-mono);
  font-size: 12px;
}

.keys__link {
  color: var(--c-primary);
}

.keys {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--c-border);
  border-radius: 10px;
  overflow: hidden;
}

.key-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
}

.key-row:not(:last-child) {
  border-bottom: 1px solid var(--c-border);
}

.key-row__icon {
  flex-shrink: 0;
  color: var(--c-text-3);
}

.key-row__main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1 1 auto;
  min-width: 0;
}

.key-row__head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}

.key-row__name {
  font-size: 13.5px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.key-row__prefix {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--c-text-2);
}

.key-row__meta {
  display: flex;
  gap: 4px 14px;
  flex-wrap: wrap;
  font-size: 12px;
}

.keys__empty {
  margin: 0;
  padding: 4px 0;
  font-size: 13px;
}

.created {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.created__key {
  display: block;
  margin-top: 6px;
  padding: 10px 12px;
  border: 1px solid var(--c-border);
  border-radius: 9px;
  background: var(--c-code-bg);
  font-family: var(--font-mono);
  font-size: 13px;
  overflow-wrap: anywhere;
  user-select: all;
}
</style>
