<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ApiError, deleteUser, listUsers, updateUser } from "../lib/api";
import { notifyError, notifySuccess } from "../lib/notify";
import { useAuthStore } from "../stores/auth";
import type { UsrMain } from "../types/api";

const router = useRouter();
const authStore = useAuthStore();

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

  const approved = window.confirm(`Delete user "${user.username}"?`);
  if (!approved) {
    return;
  }

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
  const approved = window.confirm(`${nextActive ? "Activate" : "Deactivate"} user "${user.username}"?`);
  if (!approved) {
    return;
  }

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
    <input
      v-model="searchFilter"
      class="users-filter"
      type="search"
      placeholder="Search users"
      aria-label="Search users"
      @keydown.enter.prevent="loadUsers"
    />
    <button
      class="icon-action-button secondary"
      type="button"
      :disabled="loading || saving"
      title="Refresh Users"
      aria-label="Refresh Users"
      @click="loadUsers"
    >
      <span class="icon-action-glyph">{{ loading ? "…" : "↻" }}</span>
    </button>
    <button
      class="icon-action-button primary"
      type="button"
      :disabled="saving || !canManage"
      title="Add User"
      aria-label="Add User"
      @click="goToCreate"
    >
      <span class="icon-action-glyph">＋</span>
    </button>
  </div>

  <p v-if="errorMessage" class="error">{{ errorMessage }}</p>

  <section class="panel">
    <h3>Users</h3>
    <div class="table-wrap">
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
              <span class="admin-badge" :class="{ yes: user.is_admin }">{{ user.is_admin ? "admin" : "user" }}</span>
            </td>
            <td>
              <span class="status-chip" :class="{ inactive: !user.active }">{{ user.active ? "active" : "inactive" }}</span>
            </td>
            <td>
              <div class="actions">
                <button
                  class="icon-action-button secondary"
                  type="button"
                  :disabled="saving || !canManage"
                  title="Edit User"
                  aria-label="Edit User"
                  @click="goToEdit(user)"
                >
                  <span class="icon-action-glyph">✎</span>
                </button>
                <button
                  class="icon-action-button secondary"
                  type="button"
                  :disabled="saving || !canManage"
                  :title="user.active ? 'Deactivate User' : 'Activate User'"
                  :aria-label="user.active ? 'Deactivate User' : 'Activate User'"
                  @click="toggleActive(user, !user.active)"
                >
                  <span class="icon-action-glyph">{{ user.active ? "⏸" : "▶" }}</span>
                </button>
                <button
                  class="icon-action-button danger"
                  type="button"
                  :disabled="removingId === toUserId(user.id) || saving || !canManage"
                  title="Delete User"
                  aria-label="Delete User"
                  @click="removeUser(user)"
                >
                  <span class="icon-action-glyph">{{ removingId === toUserId(user.id) ? "…" : "✕" }}</span>
                </button>
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
  </section>
</template>
