<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, ref, watch, type VNodeChild } from "vue";
import {
  NAlert,
  NButton,
  NDataTable,
  NIcon,
  NTag,
  useMessage,
  type DataTableColumns
} from "naive-ui";
import { BanOutline, RefreshOutline, ShieldOutline } from "@vicons/ionicons5";
import { listBlockedClients } from "@/api/blockedClient";
import { apiErrorMessage } from "@/api/http";
import { useAuthStore } from "@/stores/auth";
import { useIsMobile } from "@/composables/useIsMobile";
import { formatUnixDateTime, formatUnixRelative } from "@/lib/datetime";
import { BLOCKED_CLIENT_SOURCE_LABEL, BLOCKED_CLIENT_SOURCE_TAG } from "@/constants/enums";
import PageContainer from "@/components/common/PageContainer.vue";
import SectionCard from "@/components/common/SectionCard.vue";
import SwitchField from "@/components/common/SwitchField.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import BlockClientModal from "@/components/blockedclient/BlockClientModal.vue";
import UnblockClientModal from "@/components/blockedclient/UnblockClientModal.vue";
import LiftAgentBlocksModal from "@/components/blockedclient/LiftAgentBlocksModal.vue";
import type { BlockedClientMain } from "@/api/types";

const POLL_INTERVAL_MS = 30_000;

const message = useMessage();
const authStore = useAuthStore();
const isMobile = useIsMobile();
const canEdit = computed(() => authStore.isAdmin);

const items = ref<BlockedClientMain[]>([]);
const loading = ref(false);
const loaded = ref(false);
const showHistory = ref(false);
// relative times ("in 5h") are counted from this moment, moved on every refresh
const now = ref(Math.floor(Date.now() / 1000));

const showBlock = ref(false);
const showUnblock = ref(false);
const unblockRow = ref<BlockedClientMain | null>(null);
const showLiftAgent = ref(false);

let timer: ReturnType<typeof setInterval> | undefined;
let requestSeq = 0;

// Active blocks first, each group in the server order (newest first).
const rows = computed(() => [
  ...items.value.filter((row) => row.active),
  ...items.value.filter((row) => !row.active)
]);
const activeRows = computed(() => items.value.filter((row) => row.active));
const activeCount = computed(() => activeRows.value.length);
const activeAgentCount = computed(
  () => activeRows.value.filter((row) => row.source === "agent").length
);
const historyCount = computed(() => items.value.length - activeCount.value);

const emptyText = computed(() =>
  showHistory.value
    ? "No blocks yet — nothing has ever been blocked."
    : "No active blocks — every client reaches the gateway."
);

async function load(): Promise<void> {
  const seq = ++requestSeq;
  loading.value = true;
  try {
    const rep = await listBlockedClients(showHistory.value);
    // a toggle of "Show history" while a request is in flight wins
    if (seq !== requestSeq) return;
    items.value = rep.results;
    now.value = Math.floor(Date.now() / 1000);
    loaded.value = true;
  } catch (error) {
    if (seq === requestSeq) message.error(apiErrorMessage(error, "Failed to load blocked clients"));
  } finally {
    if (seq === requestSeq) loading.value = false;
  }
}

watch(showHistory, () => void load());

function openUnblock(row: BlockedClientMain): void {
  unblockRow.value = row;
  showUnblock.value = true;
}

function sourceTag(row: BlockedClientMain) {
  return h(
    NTag,
    {
      size: "small",
      bordered: false,
      type: BLOCKED_CLIENT_SOURCE_TAG[row.source],
      title: row.source === "agent" ? "Blocked by an agent with an API key (pulse, an MCP agent)" : "Blocked in the admin"
    },
    { default: () => BLOCKED_CLIENT_SOURCE_LABEL[row.source] }
  );
}

function stateTag(row: BlockedClientMain) {
  return h(
    NTag,
    { size: "tiny", bordered: false, type: "default" },
    { default: () => (row.removed_at_unix ? "lifted" : "expired") }
  );
}

function relative(unix: number): string {
  return formatUnixRelative(unix, "—", now.value);
}

function expiresText(row: BlockedClientMain): string {
  return row.expires_at_unix ? relative(row.expires_at_unix) : "no end";
}

// For a history row: when and by whom it was lifted, or that it ran out.
function endedText(row: BlockedClientMain): string {
  if (row.removed_at_unix) {
    return `lifted ${relative(row.removed_at_unix)}${row.removed_by ? ` by ${row.removed_by}` : ""}`;
  }
  return row.expires_at_unix ? `expired ${relative(row.expires_at_unix)}` : "ended";
}

function endedTitle(row: BlockedClientMain): string {
  return formatUnixDateTime(row.removed_at_unix || row.expires_at_unix, "");
}

function renderEnd(row: BlockedClientMain): VNodeChild {
  if (row.active) {
    if (!row.expires_at_unix) {
      return h(NTag, { size: "small", bordered: false, type: "error" }, { default: () => "no end" });
    }
    return h("span", { title: formatUnixDateTime(row.expires_at_unix) }, expiresText(row));
  }
  return h("div", { class: "bc-ended" }, [
    h("span", { title: endedTitle(row) }, endedText(row)),
    row.remove_reason
      ? h("span", { class: "bc-ended__reason", title: row.remove_reason }, row.remove_reason)
      : null
  ]);
}

const columns = computed<DataTableColumns<BlockedClientMain>>(() => {
  const list: DataTableColumns<BlockedClientMain> = [
    {
      title: "Address",
      key: "ip",
      width: 200,
      render: (row) =>
        h("div", { class: "bc-address" }, [
          h("span", { class: "mono" }, row.ip || "—"),
          row.active ? null : stateTag(row)
        ])
    },
    {
      title: "Reason",
      key: "reason",
      minWidth: 200,
      ellipsis: { tooltip: true },
      render: (row) => row.reason || "—"
    },
    {
      title: "Blocked by",
      key: "created_by",
      width: 210,
      render: (row) =>
        h("div", { class: "bc-who" }, [
          sourceTag(row),
          h("span", { class: "bc-who__name", title: row.created_by }, row.created_by || "—")
        ])
    },
    {
      title: "Created",
      key: "created_at_unix",
      width: 110,
      render: (row) =>
        h(
          "span",
          { class: "muted", title: formatUnixDateTime(row.created_at_unix) },
          relative(row.created_at_unix)
        )
    },
    {
      title: showHistory.value ? "Expires / ended" : "Expires",
      key: "expires_at_unix",
      width: showHistory.value ? 220 : 120,
      render: renderEnd
    }
  ];
  if (canEdit.value) {
    list.push({
      title: "",
      key: "actions",
      width: 110,
      align: "right",
      render: (row) =>
        row.active
          ? h(
              NButton,
              { size: "small", secondary: true, onClick: () => openUnblock(row) },
              { default: () => "Unblock" }
            )
          : null
    });
  }
  return list;
});

function rowClassName(row: BlockedClientMain): string {
  return row.active ? "" : "bc-row--history";
}

onMounted(() => {
  void load();
  timer = setInterval(() => {
    now.value = Math.floor(Date.now() / 1000);
    if (document.hidden || loading.value) return;
    void load();
  }, POLL_INTERVAL_MS);
});

onBeforeUnmount(() => {
  clearInterval(timer);
});
</script>

<template>
  <PageContainer :width="1180">
    <div class="page-head">
      <div>
        <h1 class="page-head__title">Blocked clients</h1>
        <p class="page-head__sub muted">
          Addresses and networks the gateways refuse with <span class="mono">403</span> on every
          route · {{ activeCount }} active
        </p>
      </div>
      <div class="page-head__actions">
        <NButton tertiary :loading="loading" @click="load">
          <template #icon><NIcon :component="RefreshOutline" /></template>
          Refresh
        </NButton>
        <NButton
          v-if="canEdit && activeAgentCount > 0"
          secondary
          type="warning"
          title="Kill switch: lift the active blocks made by agents"
          @click="showLiftAgent = true"
        >
          <template #icon><NIcon :component="ShieldOutline" /></template>
          Lift agent blocks ({{ activeAgentCount }})
        </NButton>
        <NButton v-if="canEdit" type="error" @click="showBlock = true">
          <template #icon><NIcon :component="BanOutline" /></template>
          Block address
        </NButton>
      </div>
    </div>

    <NAlert type="info" :bordered="false">
      A block applies on all gateways within seconds, no Deploy needed. Blocks made with an API key
      (source <strong>agent</strong>: pulse or an MCP agent) always have an end — at most 7 days —
      never touch internal networks, and an agent can lift only agent blocks, not a person's. Trusted clients from the Root configuration can never be
      blocked.
      <template v-if="!canEdit"> Administrator rights are required to block or unblock.</template>
    </NAlert>

    <SectionCard>
      <div class="bc-toolbar">
        <SwitchField v-model="showHistory" label="Show history" />
        <span v-if="showHistory && loaded" class="muted bc-toolbar__count">
          {{ historyCount }} lifted or expired
        </span>
      </div>

      <template v-if="isMobile">
        <div v-if="rows.length" class="bc-cards">
          <div
            v-for="row in rows"
            :key="row.id"
            class="bc-card"
            :class="{ 'bc-card--history': !row.active }"
          >
            <div class="bc-card__head">
              <span class="bc-card__ip mono">{{ row.ip }}</span>
              <NTag
                size="small"
                :bordered="false"
                :type="BLOCKED_CLIENT_SOURCE_TAG[row.source]"
              >
                {{ BLOCKED_CLIENT_SOURCE_LABEL[row.source] }}
              </NTag>
            </div>
            <p class="bc-card__reason">{{ row.reason || "—" }}</p>
            <div class="bc-card__meta muted">
              <span>by {{ row.created_by || "—" }}</span>
              <span :title="formatUnixDateTime(row.created_at_unix)">
                created {{ relative(row.created_at_unix) }}
              </span>
              <span v-if="row.active" :title="formatUnixDateTime(row.expires_at_unix, '')">
                {{ row.expires_at_unix ? `expires ${expiresText(row)}` : "no end" }}
              </span>
              <span v-else :title="endedTitle(row)">{{ endedText(row) }}</span>
            </div>
            <p v-if="!row.active && row.remove_reason" class="bc-card__reason muted">
              {{ row.remove_reason }}
            </p>
            <NButton
              v-if="canEdit && row.active"
              size="small"
              secondary
              class="bc-card__action"
              @click="openUnblock(row)"
            >
              Unblock
            </NButton>
          </div>
        </div>
        <EmptyState v-else-if="loaded" :description="emptyText" size="medium" />
      </template>
      <NDataTable
        v-else
        :columns="columns"
        :data="rows"
        :loading="loading && !loaded"
        :row-key="(row: BlockedClientMain) => row.id"
        :row-class-name="rowClassName"
        :bordered="false"
        :scroll-x="960"
      >
        <template #empty>
          <EmptyState :description="emptyText" size="medium" />
        </template>
      </NDataTable>
    </SectionCard>

    <BlockClientModal v-model:show="showBlock" :active="activeRows" @saved="load" />
    <UnblockClientModal v-model:show="showUnblock" :row="unblockRow" @removed="load" />
    <LiftAgentBlocksModal v-model:show="showLiftAgent" :active="activeRows" @removed="load" />
  </PageContainer>
</template>

<style scoped>
.page-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.page-head__title {
  margin: 0;
  font-size: 22px;
  font-weight: 700;
}

.page-head__sub {
  margin: 3px 0 0;
  font-size: 13px;
}

.page-head__actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.bc-toolbar {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 16px;
}

.bc-toolbar__count {
  font-size: 12.5px;
}

:deep(.bc-address) {
  display: flex;
  align-items: center;
  gap: 8px;
}

:deep(.bc-who) {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

:deep(.bc-who__name) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:deep(.bc-ended) {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

:deep(.bc-ended__reason) {
  font-size: 12px;
  color: var(--c-text-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* lifted and expired blocks stay readable but step back from the active ones */
:deep(.bc-row--history td) {
  color: var(--c-text-3);
}

:deep(.bc-row--history .n-tag) {
  opacity: 0.7;
}

.bc-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.bc-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px;
  border: 1px solid var(--c-border);
  border-radius: 11px;
  background: var(--c-surface);
}

.bc-card--history {
  opacity: 0.6;
}

.bc-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.bc-card__ip {
  font-size: 14px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.bc-card__reason {
  margin: 0;
  font-size: 13px;
  overflow-wrap: anywhere;
}

.bc-card__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 12px;
  font-size: 12.5px;
}

.bc-card__action {
  align-self: flex-start;
}
</style>
