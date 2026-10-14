<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, watch } from "vue";
import {
  NButton,
  NDataTable,
  NIcon,
  NPagination,
  NSelect,
  NTag,
  useMessage,
  type DataTableColumns,
  type SelectOption
} from "naive-ui";
import { RefreshOutline } from "@vicons/ionicons5";
import { listChangeRequests } from "@/api/changeRequest";
import { apiErrorMessage } from "@/api/http";
import { useChangeRequestsStore } from "@/stores/changeRequests";
import { useIsMobile } from "@/composables/useIsMobile";
import { formatUnixDateTime } from "@/lib/datetime";
import {
  CHANGE_REQUEST_STATUS_LABEL,
  changeRequestStatusLabel,
  changeRequestStatusTag
} from "@/lib/changeRequest";
import PageContainer from "@/components/common/PageContainer.vue";
import SectionCard from "@/components/common/SectionCard.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import ChangeRequestDetailDrawer from "@/components/changerequest/ChangeRequestDetailDrawer.vue";
import type { ChangeRequestMain, ChangeRequestStatus } from "@/api/types";

type StatusFilter = ChangeRequestStatus | "all";

const message = useMessage();
const changeRequestsStore = useChangeRequestsStore();
const isMobile = useIsMobile();

const rows = ref<ChangeRequestMain[]>([]);
const loading = ref(false);
const status = ref<StatusFilter>("pending");

const statusOptions: SelectOption[] = [
  { label: "All statuses", value: "all" },
  ...Object.entries(CHANGE_REQUEST_STATUS_LABEL).map(([value, label]) => ({ label, value }))
];

const pagination = reactive({
  page: 1,
  pageSize: 50,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [20, 50, 100]
});

const showDetail = ref(false);
const detailId = ref<number | null>(null);

async function fetchRequests(): Promise<void> {
  loading.value = true;
  try {
    const rep = await listChangeRequests({
      page: pagination.page - 1,
      page_size: pagination.pageSize,
      with_total_count: true,
      status: status.value === "all" ? undefined : status.value
    });
    rows.value = rep.results;
    pagination.itemCount = Number(rep.pagination_info?.total_count ?? 0);
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to load change requests"));
  } finally {
    loading.value = false;
  }
}

function refresh(): void {
  void fetchRequests();
  void changeRequestsStore.loadPendingCount();
}

function onChanged(): void {
  refresh();
}

watch(status, () => {
  pagination.page = 1;
  void fetchRequests();
});

// The navigation badge polls the pending count; a new request arriving while
// this page is open shows up in the list too.
watch(
  () => changeRequestsStore.pendingCount,
  () => {
    if (!loading.value) void fetchRequests();
  }
);

function openDetail(row: ChangeRequestMain): void {
  detailId.value = row.id;
  showDetail.value = true;
}

function statusTag(row: ChangeRequestMain) {
  return h(
    NTag,
    { size: "small", bordered: false, type: changeRequestStatusTag(row.status) },
    { default: () => changeRequestStatusLabel(row.status) }
  );
}

const columns = computed<DataTableColumns<ChangeRequestMain>>(() => [
  {
    title: "#",
    key: "id",
    width: 70,
    render: (row) => h("span", { class: "mono muted" }, `#${row.id}`)
  },
  {
    title: "Title",
    key: "title",
    render: (row) =>
      h(
        NButton,
        { text: true, type: "primary", class: "cr-title-btn", onClick: () => openDetail(row) },
        { default: () => row.title || "Untitled" }
      )
  },
  {
    title: "Status",
    key: "status",
    width: 110,
    render: statusTag
  },
  {
    title: "Changes",
    key: "ops_count",
    width: 90,
    align: "right",
    render: (row) => h("span", { class: "mono" }, String(row.ops_count))
  },
  {
    title: "Key",
    key: "api_key_name",
    width: 160,
    ellipsis: { tooltip: true },
    render: (row) => h("span", { class: "muted" }, row.api_key_name || "—")
  },
  {
    title: "Created",
    key: "created_at_unix",
    width: 170,
    render: (row) => h("span", { class: "muted" }, formatUnixDateTime(row.created_at_unix, "—"))
  }
]);

const rowProps = (row: ChangeRequestMain) => ({
  style: "cursor: pointer",
  onClick: () => openDetail(row)
});

const emptyText = computed(() =>
  status.value === "pending" ? "No pending change requests" : "No change requests"
);

function onPageChange(page: number): void {
  pagination.page = page;
  void fetchRequests();
}

function onPageSizeChange(size: number): void {
  pagination.pageSize = size;
  pagination.page = 1;
  void fetchRequests();
}

onMounted(fetchRequests);
</script>

<template>
  <PageContainer :width="1080">
    <div class="page-head">
      <div>
        <h1 class="page-head__title">Change requests</h1>
        <p class="page-head__sub muted">
          Configuration changes proposed by AI agents over MCP, waiting for review
        </p>
      </div>
      <NButton tertiary :loading="loading" @click="refresh">
        <template #icon><NIcon :component="RefreshOutline" /></template>
        Refresh
      </NButton>
    </div>

    <SectionCard>
      <NSelect
        v-model:value="status"
        :options="statusOptions"
        class="cr-list__filter"
        aria-label="Status"
      />

      <template v-if="isMobile">
        <div v-if="rows.length" class="cr-cards">
          <button
            v-for="row in rows"
            :key="row.id"
            type="button"
            class="cr-card"
            @click="openDetail(row)"
          >
            <div class="cr-card__head">
              <span class="cr-card__title">
                <span class="mono muted">#{{ row.id }}</span>
                {{ row.title || "Untitled" }}
              </span>
              <NTag size="small" :bordered="false" :type="changeRequestStatusTag(row.status)">
                {{ changeRequestStatusLabel(row.status) }}
              </NTag>
            </div>
            <div class="cr-card__meta muted">
              <span>{{ row.ops_count }} change{{ row.ops_count === 1 ? "" : "s" }}</span>
              <span v-if="row.api_key_name">key «{{ row.api_key_name }}»</span>
              <span>{{ formatUnixDateTime(row.created_at_unix, "—") }}</span>
            </div>
          </button>
        </div>
        <EmptyState v-else-if="!loading" :description="emptyText" size="medium" />
        <div v-if="pagination.itemCount > pagination.pageSize" class="cr-pager">
          <NPagination
            :page="pagination.page"
            :page-size="pagination.pageSize"
            :item-count="pagination.itemCount"
            @update:page="onPageChange"
          />
        </div>
      </template>
      <NDataTable
        v-else
        remote
        :columns="columns"
        :data="rows"
        :loading="loading"
        :pagination="pagination"
        :row-props="rowProps"
        :row-key="(row: ChangeRequestMain) => row.id"
        :bordered="false"
        :scroll-x="760"
        @update:page="onPageChange"
        @update:page-size="onPageSizeChange"
      >
        <template #empty>
          <EmptyState :description="emptyText" size="medium" />
        </template>
      </NDataTable>
    </SectionCard>

    <ChangeRequestDetailDrawer
      v-model:show="showDetail"
      :request-id="detailId"
      @changed="onChanged"
    />
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

.cr-list__filter {
  max-width: 220px;
  margin-bottom: 16px;
}

:deep(.cr-title-btn) {
  white-space: normal;
  text-align: left;
}

.cr-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.cr-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px;
  border: 1px solid var(--c-border);
  border-radius: 11px;
  background: var(--c-surface);
  color: var(--c-text);
  text-align: left;
  font: inherit;
  cursor: pointer;
  transition: border-color 0.14s ease;
}

.cr-card:hover {
  border-color: var(--c-border-strong);
}

.cr-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.cr-card__title {
  font-size: 14.5px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.cr-card__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 12px;
  font-size: 12.5px;
}

.cr-pager {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}
</style>
