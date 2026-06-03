<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useDialog } from "naive-ui";
import { CreateOutline, PauseCircleOutline, PersonAddOutline, PlayCircleOutline, RefreshOutline, SearchOutline, TrashOutline } from "@vicons/ionicons5";
import { ApiError, deleteUser, listUsers, updateUser } from "../lib/api";
import { notifyError, notifySuccess } from "../lib/notify";
import { useAuthStore } from "../stores/auth";
import type { UsrMain } from "../types/api";

const router = useRouter();
const authStore = useAuthStore();
const dialog = useDialog();

const users = ref<UsrMain[]>([]);
const loading = ref(false);
const saving = ref(false);
const removingId = ref<number | null>(null);
const errorMessage = ref("");
const searchFilter = ref("");

const canManage = computed(() => Boolean(authStore.profile?.is_admin));

function toUserId(value: unknown): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return 0;
}

async function loadUsers() {
  loading.value = true;
  errorMessage.value = "";
  try {
    const rep = await listUsers({
      page: 0,
      page_size: 200,
      with_total_count: true,
      search: searchFilter.value.trim() || undefined
    });
    users.value = rep.results || [];
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load users";
    notifyError(errorMessage.value);
  } finally {
    loading.value = false;
  }
}

function goToCreate() {
  void router.push({ name: "users-create" });
}

function goToEdit(user: UsrMain) {
  void router.push({ name: "users-edit", params: { id: toUserId(user.id) } });
}

async function removeUser(user: UsrMain) {
  if (!canManage.value) {
    notifyError("Admin permissions are required");
    return;
  }

  const userId = toUserId(user.id);
  if (!userId) {
    notifyError("Invalid user id");
    return;
  }

  dialog.error({
    title: "Delete user",
    content: `Delete user "${user.username}"?`,
    positiveText: "Delete",
    negativeText: "Cancel",
    onPositiveClick: () => {
      void runRemoveUser(user, userId);
    }
  });
}

async function runRemoveUser(user: UsrMain, userId: number) {
  removingId.value = userId;
  errorMessage.value = "";
  try {
    await deleteUser(userId);
    notifySuccess("User deleted");
    await loadUsers();
  } catch (error) {
    if (error instanceof ApiError && error.status === 403) {
      notifyError("Only admin can delete users");
    } else {
      errorMessage.value = error instanceof Error ? error.message : "Unable to delete user";
      notifyError(errorMessage.value);
    }
  } finally {
    removingId.value = null;
  }
}

async function toggleActive(user: UsrMain, nextActive: boolean) {
  if (!canManage.value) {
    notifyError("Admin permissions are required");
    return;
  }
  dialog.warning({
    title: nextActive ? "Activate user" : "Deactivate user",
    content: `${nextActive ? "Activate" : "Deactivate"} user "${user.username}"?`,
    positiveText: nextActive ? "Activate" : "Deactivate",
    negativeText: "Cancel",
    onPositiveClick: () => {
      void runToggleActive(user, nextActive);
    }
  });
}

async function runToggleActive(user: UsrMain, nextActive: boolean) {
  saving.value = true;
  errorMessage.value = "";
  try {
    await updateUser({
      id: toUserId(user.id),
      active: nextActive
    });
    notifySuccess(nextActive ? "User activated" : "User deactivated");
    await loadUsers();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to update user";
    notifyError(errorMessage.value);
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  void loadUsers();
});
</script>

<template>
  <div class="actions page-top-actions users-top-actions">
    <n-input
      v-model:value="searchFilter"
      class="users-filter"
      placeholder="Search users"
      aria-label="Search users"
      clearable
      @keydown.enter.prevent="loadUsers"
    >
      <template #prefix>
        <n-icon :component="SearchOutline" />
      </template>
    </n-input>
    <n-button
      secondary
      :loading="loading"
      :disabled="saving"
      title="Refresh Users"
      aria-label="Refresh Users"
      @click="loadUsers"
    >
      <n-icon :component="RefreshOutline" />
    </n-button>
    <n-button
      type="primary"
      :disabled="saving || !canManage"
      title="Add User"
      aria-label="Add User"
      @click="goToCreate"
    >
      <n-icon :component="PersonAddOutline" />
    </n-button>
  </div>

  <n-alert v-if="errorMessage" class="form-alert" type="error" :show-icon="false">{{ errorMessage }}</n-alert>

  <section class="panel">
    <h3>Users</h3>
    <div class="table-wrap users-table-wrap">
      <table class="data-table users-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>Username</th>
            <th>Name</th>
            <th>Role</th>
            <th>Status</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users" :key="toUserId(user.id)">
            <td>{{ toUserId(user.id) }}</td>
            <td>{{ user.username }}</td>
            <td>{{ user.name }}</td>
            <td>
              <n-tag size="small" :type="user.is_admin ? 'success' : 'default'">{{ user.is_admin ? "admin" : "user" }}</n-tag>
            </td>
            <td>
              <n-tag size="small" :type="user.active ? 'success' : 'warning'">{{ user.active ? "active" : "inactive" }}</n-tag>
            </td>
            <td>
              <div class="actions">
                <n-button
                  secondary
                  size="small"
                  :disabled="saving || !canManage"
                  title="Edit User"
                  aria-label="Edit User"
                  @click="goToEdit(user)"
                >
                  <n-icon :component="CreateOutline" />
                </n-button>
                <n-button
                  secondary
                  size="small"
                  :disabled="saving || !canManage"
                  :title="user.active ? 'Deactivate User' : 'Activate User'"
                  :aria-label="user.active ? 'Deactivate User' : 'Activate User'"
                  @click="toggleActive(user, !user.active)"
                >
                  <n-icon :component="user.active ? PauseCircleOutline : PlayCircleOutline" />
                </n-button>
                <n-button
                  class="danger-icon-button"
                  type="error"
                  secondary
                  size="small"
                  circle
                  :disabled="removingId === toUserId(user.id) || saving || !canManage"
                  title="Delete User"
                  aria-label="Delete User"
                  @click="removeUser(user)"
                >
                  <n-icon v-if="removingId !== toUserId(user.id)" :component="TrashOutline" />
                </n-button>
              </div>
            </td>
          </tr>
          <tr v-if="!loading && users.length === 0">
            <td colspan="6" class="muted">No users found.</td>
          </tr>
          <tr v-if="loading && users.length === 0">
            <td colspan="6" class="muted">Loading users...</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="users-mobile-list">
      <p v-if="!loading && users.length === 0" class="muted">No users found.</p>
      <p v-if="loading && users.length === 0" class="muted">Loading users...</p>
      <article v-for="user in users" :key="`mobile-${toUserId(user.id)}`" class="users-mobile-card">
        <div class="users-mobile-head">
          <strong class="users-mobile-username">{{ user.username }}</strong>
          <n-tag size="small" :type="user.active ? 'success' : 'warning'">{{ user.active ? "active" : "inactive" }}</n-tag>
        </div>
        <div class="users-mobile-grid">
          <div class="users-mobile-row">
            <span class="label">ID</span>
            <span>{{ toUserId(user.id) }}</span>
          </div>
          <div class="users-mobile-row">
            <span class="label">Name</span>
            <span>{{ user.name || "-" }}</span>
          </div>
          <div class="users-mobile-row">
            <span class="label">Role</span>
            <n-tag size="small" :type="user.is_admin ? 'success' : 'default'">{{ user.is_admin ? "admin" : "user" }}</n-tag>
          </div>
        </div>
        <div class="users-mobile-actions">
          <n-button
            secondary
            size="small"
            :disabled="saving || !canManage"
            title="Edit User"
            aria-label="Edit User"
            @click="goToEdit(user)"
          >
            <n-icon :component="CreateOutline" />
          </n-button>
          <n-button
            secondary
            size="small"
            :disabled="saving || !canManage"
            :title="user.active ? 'Deactivate User' : 'Activate User'"
            :aria-label="user.active ? 'Deactivate User' : 'Activate User'"
            @click="toggleActive(user, !user.active)"
          >
            <n-icon :component="user.active ? PauseCircleOutline : PlayCircleOutline" />
          </n-button>
          <n-button
            class="danger-icon-button"
            type="error"
            secondary
            size="small"
            circle
            :disabled="removingId === toUserId(user.id) || saving || !canManage"
            title="Delete User"
            aria-label="Delete User"
            @click="removeUser(user)"
          >
            <n-icon v-if="removingId !== toUserId(user.id)" :component="TrashOutline" />
          </n-button>
        </div>
      </article>
    </div>
  </section>
</template>
