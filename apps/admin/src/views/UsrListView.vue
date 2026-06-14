<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, watch } from "vue";
import {
  NButton,
  NDataTable,
  NIcon,
  NInput,
  NPagination,
  NPopconfirm,
  NTag,
  useMessage,
  type DataTableColumns
} from "naive-ui";
import {
  AddOutline,
  CreateOutline,
  PauseOutline,
  PlayOutline,
  SearchOutline,
  TrashOutline
} from "@vicons/ionicons5";
import { deleteUser, listUsers, updateUser } from "@/api/usr";
import { apiErrorMessage } from "@/api/http";
import { useAuthStore } from "@/stores/auth";
import { useIsMobile } from "@/composables/useIsMobile";
import PageContainer from "@/components/common/PageContainer.vue";
import SectionCard from "@/components/common/SectionCard.vue";
import UsrFormModal from "@/components/usr/UsrFormModal.vue";
import UsrDetailDrawer from "@/components/usr/UsrDetailDrawer.vue";
import type { UsrMain } from "@/api/types";

const message = useMessage();
const authStore = useAuthStore();
const isMobile = useIsMobile();

const rows = ref<UsrMain[]>([]);
const loading = ref(false);
const search = ref("");

const pagination = reactive({
  page: 1,
  pageSize: 20,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50]
});

const showForm = ref(false);
const editing = ref<UsrMain | null>(null);
const showDetail = ref(false);
const detailId = ref<number | null>(null);

const selfId = computed(() => authStore.profile?.id);

let searchTimer: ReturnType<typeof setTimeout> | undefined;

async function fetchUsers(): Promise<void> {
  loading.value = true;
  try {
    const rep = await listUsers({
      page: pagination.page - 1,
      page_size: pagination.pageSize,
      with_total_count: true,
      search: search.value.trim() || undefined
    });
    rows.value = rep.results ?? [];
    pagination.itemCount = Number(rep.pagination_info?.total_count ?? 0);
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to load users"));
  } finally {
    loading.value = false;
  }
}

watch(search, () => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    pagination.page = 1;
    void fetchUsers();
  }, 350);
});

function openCreate(): void {
  editing.value = null;
  showForm.value = true;
}

function openEdit(user: UsrMain): void {
  editing.value = user;
  showForm.value = true;
}

function openDetail(user: UsrMain): void {
  detailId.value = user.id;
  showDetail.value = true;
}

async function toggleActive(user: UsrMain): Promise<void> {
  try {
    await updateUser({ id: user.id, active: !user.active });
    message.success(user.active ? "User deactivated" : "User activated");
    await fetchUsers();
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to update user"));
  }
}

async function removeUser(user: UsrMain): Promise<void> {
  try {
    await deleteUser(user.id);
    message.success("User deleted");
    await fetchUsers();
  } catch (error) {
    message.error(apiErrorMessage(error, "Failed to delete user"));
  }
}

function accessLabel(user: UsrMain): string {
  if (user.is_admin || user.all_apps) return "All applications";
  if (user.app_ids.length) return `${user.app_ids.length} application(s)`;
  return "—";
}

const columns = computed<DataTableColumns<UsrMain>>(() => [
  {
    title: "Name",
    key: "name",
    render: (row) =>
      h(
        NButton,
        { text: true, type: "primary", onClick: () => openDetail(row) },
        { default: () => row.name || row.username }
      )
  },
  {
    title: "Username",
    key: "username",
    render: (row) => h("span", { class: "mono" }, row.username)
  },
  {
    title: "Role",
    key: "is_admin",
    width: 130,
    render: (row) =>
      h(
        NTag,
        { size: "small", bordered: false, type: row.is_admin ? "success" : "default" },
        { default: () => (row.is_admin ? "Administrator" : "User") }
      )
  },
  {
    title: "Access",
    key: "access",
    render: (row) => h("span", { class: "muted" }, accessLabel(row))
  },
  {
    title: "Status",
    key: "active",
    width: 110,
    render: (row) =>
      h(
        NTag,
        { size: "small", bordered: false, type: row.active ? "success" : "default" },
        { default: () => (row.active ? "Active" : "Inactive") }
      )
  },
  {
    title: "",
    key: "actions",
    width: 150,
    align: "right",
    render: (row) =>
      h("div", { class: "row-actions" }, [
        h(
          NButton,
          {
            quaternary: true,
            circle: true,
            size: "small",
            title: "Edit",
            onClick: () => openEdit(row)
          },
          { icon: () => h(NIcon, null, { default: () => h(CreateOutline) }) }
        ),
        h(
          NButton,
          {
            quaternary: true,
            circle: true,
            size: "small",
            title: row.active ? "Deactivate" : "Activate",
            onClick: () => toggleActive(row)
          },
          {
            icon: () => h(NIcon, null, { default: () => h(row.active ? PauseOutline : PlayOutline) })
          }
        ),
        row.id === selfId.value
          ? h(
              NButton,
              {
                quaternary: true,
                circle: true,
                size: "small",
                type: "error",
                disabled: true,
                title: "You cannot delete yourself"
              },
              { icon: () => h(NIcon, null, { default: () => h(TrashOutline) }) }
            )
          : h(
              NPopconfirm,
              { onPositiveClick: () => removeUser(row) },
              {
                trigger: () =>
                  h(
                    NButton,
                    {
                      class: "danger-icon-button",
                      quaternary: true,
                      circle: true,
                      size: "small",
                      type: "error",
                      title: "Delete"
                    },
                    { icon: () => h(NIcon, null, { default: () => h(TrashOutline) }) }
                  ),
                default: () => `Delete "${row.username}"?`
              }
            )
      ])
  }
]);

function onPageChange(page: number): void {
  pagination.page = page;
  void fetchUsers();
}

function onPageSizeChange(size: number): void {
  pagination.pageSize = size;
  pagination.page = 1;
  void fetchUsers();
}

onMounted(fetchUsers);
</script>

<template>
  <PageContainer :width="1080">
    <div class="page-head">
      <div>
        <h1 class="page-head__title">Users</h1>
        <p class="page-head__sub muted">Administrators and scoped read-only operators</p>
      </div>
      <NButton type="primary" @click="openCreate">
        <template #icon><NIcon :component="AddOutline" /></template>
        New user
      </NButton>
    </div>

    <SectionCard>
      <NInput
        v-model:value="search"
        placeholder="Search by name or username"
        clearable
        class="users__search"
      >
        <template #prefix><NIcon :component="SearchOutline" /></template>
      </NInput>

      <template v-if="isMobile">
        <div v-if="rows.length" class="usr-cards">
          <div v-for="user in rows" :key="user.id" class="usr-card">
            <div class="usr-card__head">
              <button type="button" class="usr-card__name" @click="openDetail(user)">
                {{ user.name || user.username }}
              </button>
              <NTag
                size="small"
                :bordered="false"
                :type="user.active ? 'success' : 'default'"
              >
                {{ user.active ? "Active" : "Inactive" }}
              </NTag>
            </div>
            <div class="usr-card__username mono">{{ user.username }}</div>
            <div class="usr-card__tags">
              <NTag size="small" :bordered="false" :type="user.is_admin ? 'success' : 'default'">
                {{ user.is_admin ? "Administrator" : "User" }}
              </NTag>
              <span class="usr-card__access muted">{{ accessLabel(user) }}</span>
            </div>
            <div class="usr-card__actions">
              <NButton quaternary circle size="small" title="Edit" @click="openEdit(user)">
                <template #icon><NIcon :component="CreateOutline" /></template>
              </NButton>
              <NButton
                quaternary
                circle
                size="small"
                :title="user.active ? 'Deactivate' : 'Activate'"
                @click="toggleActive(user)"
              >
                <template #icon>
                  <NIcon :component="user.active ? PauseOutline : PlayOutline" />
                </template>
              </NButton>
              <NButton
                v-if="user.id === selfId"
                class="danger-icon-button"
                quaternary
                circle
                size="small"
                type="error"
                disabled
                title="You cannot delete yourself"
              >
                <template #icon><NIcon :component="TrashOutline" /></template>
              </NButton>
              <NPopconfirm v-else @positive-click="removeUser(user)">
                <template #trigger>
                  <NButton
                    class="danger-icon-button"
                    quaternary
                    circle
                    size="small"
                    type="error"
                    title="Delete"
                  >
                    <template #icon><NIcon :component="TrashOutline" /></template>
                  </NButton>
                </template>
                Delete "{{ user.username }}"?
              </NPopconfirm>
            </div>
          </div>
        </div>
        <p v-else-if="!loading" class="muted usr-empty">No users found.</p>
        <div v-if="pagination.itemCount > pagination.pageSize" class="usr-pager">
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
        :bordered="false"
        :scroll-x="760"
        @update:page="onPageChange"
        @update:page-size="onPageSizeChange"
      />
    </SectionCard>

    <UsrFormModal v-model:show="showForm" :user="editing" @saved="fetchUsers" />
    <UsrDetailDrawer v-model:show="showDetail" :user-id="detailId" />
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

.users__search {
  max-width: 320px;
  margin-bottom: 16px;
}

:deep(.row-actions) {
  display: flex;
  justify-content: flex-end;
  gap: 2px;
}

.usr-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.usr-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px;
  border: 1px solid var(--c-border);
  border-radius: 11px;
  background: var(--c-surface);
}

.usr-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.usr-card__name {
  padding: 0;
  border: none;
  background: none;
  color: var(--c-primary);
  font-size: 14.5px;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
  overflow-wrap: anywhere;
}

.usr-card__username {
  font-size: 12.5px;
  color: var(--c-text-2);
  overflow-wrap: anywhere;
}

.usr-card__tags {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.usr-card__access {
  font-size: 12.5px;
}

.usr-card__actions {
  display: flex;
  justify-content: flex-end;
  gap: 4px;
  margin-top: 2px;
  border-top: 1px solid var(--c-border);
  padding-top: 8px;
}

.usr-empty {
  padding: 8px 0;
}

.usr-pager {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}
</style>
