<script setup lang="ts">
import { onMounted, ref } from "vue";
import { NButton, NIcon, NSpin, useMessage } from "naive-ui";
import { LinkOutline, TrashOutline } from "@vicons/ionicons5";
import { deleteOAuthGrant, listOAuthGrants } from "@/api/oauth";
import { apiErrorMessage } from "@/api/http";
import { useConfirm } from "@/composables/useConfirm";
import { formatUnixAge, formatUnixDateTime } from "@/lib/datetime";
import SectionCard from "@/components/common/SectionCard.vue";
import type { OAuthGrantMain } from "@/api/types";

const message = useMessage();
const { confirmDelete } = useConfirm();

const grants = ref<OAuthGrantMain[]>([]);
const loading = ref(false);

async function fetchGrants(): Promise<void> {
  loading.value = true;
  try {
    const rep = await listOAuthGrants();
    grants.value = rep.results;
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to load connected apps"));
  } finally {
    loading.value = false;
  }
}

function remove(grant: OAuthGrantMain): void {
  confirmDelete({
    title: "Disconnect app",
    content: `Disconnect "${grant.client_name}"? It loses access immediately and has to be authorized again.`,
    positiveText: "Disconnect",
    onConfirm: async () => {
      try {
        await deleteOAuthGrant(grant.id);
        message.success("App disconnected");
        await fetchGrants();
      } catch (error) {
        message.error(apiErrorMessage(error, "Failed to disconnect the app"));
      }
    }
  });
}

onMounted(fetchGrants);
</script>

<template>
  <SectionCard
    title="Connected apps"
    description="Apps you authorized to the MCP server by signing in, e.g. Claude connectors"
  >
    <NSpin :show="loading">
      <div v-if="grants.length" class="grants">
        <div v-for="grant in grants" :key="grant.id" class="grant-row">
          <NIcon :component="LinkOutline" class="grant-row__icon" />
          <div class="grant-row__main">
            <div class="grant-row__head">
              <span class="grant-row__name">{{ grant.client_name }}</span>
              <code v-if="grant.redirect_host" class="grant-row__host">{{ grant.redirect_host }}</code>
            </div>
            <div class="grant-row__meta muted">
              <span>Connected {{ formatUnixDateTime(grant.connected_at_unix, "—") }}</span>
              <span :title="formatUnixDateTime(grant.last_used_at_unix, '')">
                Last used {{ formatUnixAge(grant.last_used_at_unix, "never") }}
              </span>
            </div>
          </div>
          <NButton
            class="danger-icon-button"
            quaternary
            circle
            size="small"
            type="error"
            title="Disconnect"
            @click="remove(grant)"
          >
            <template #icon><NIcon :component="TrashOutline" /></template>
          </NButton>
        </div>
      </div>
      <p v-else-if="!loading" class="muted grants__empty">No connected apps.</p>
    </NSpin>
  </SectionCard>
</template>

<style scoped>
.grants {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--c-border);
  border-radius: 10px;
  overflow: hidden;
}

.grant-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
}

.grant-row:not(:last-child) {
  border-bottom: 1px solid var(--c-border);
}

.grant-row__icon {
  flex-shrink: 0;
  color: var(--c-text-3);
}

.grant-row__main {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1 1 auto;
  min-width: 0;
}

.grant-row__head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}

.grant-row__name {
  font-size: 13.5px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.grant-row__host {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--c-text-2);
}

.grant-row__meta {
  display: flex;
  gap: 4px 14px;
  flex-wrap: wrap;
  font-size: 12px;
}

.grants__empty {
  margin: 0;
  padding: 4px 0;
  font-size: 13px;
}
</style>
