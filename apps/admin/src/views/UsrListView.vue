<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, watch } from "vue";
import {
  NButton,
  NDataTable,
  NIcon,
  NInput,
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
import PageContainer from "@/components/common/PageContainer.vue";
import SectionCard from "@/components/common/SectionCard.vue";
import UsrFormModal from "@/components/usr/UsrFormModal.vue";
import UsrDetailDrawer from "@/components/usr/UsrDetailDrawer.vue";
import type { UsrMain } from "@/api/types";

const message = useMessage();
const authStore = useAuthStore();

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

      <NDataTable
        remote
        :columns="columns"
        :data="rows"
        :loading="loading"
        :pagination="pagination"
        :bordered="false"
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
</style>
