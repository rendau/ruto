<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import {
  NAlert,
  NButton,
  NCard,
  NForm,
  NFormItem,
  NInput,
  NSpin,
  type FormInst,
  type FormRules
} from "naive-ui";
import { useAuthStore } from "@/stores/auth";
import { getBootstrapStatus, createUser, login as apiLogin } from "@/api/usr";
import { apiErrorMessage } from "@/api/http";
import BrandLogo from "@/components/common/BrandLogo.vue";

const authStore = useAuthStore();
const route = useRoute();
const router = useRouter();

const formRef = ref<FormInst | null>(null);
const model = reactive({ name: "", username: "", password: "" });
const errorMessage = ref("");
const statusLoading = ref(true);
const bootstrapAvailable = ref(false);
const submitting = ref(false);

const loginRules: FormRules = {
  username: [{ required: true, message: "Username is required", trigger: ["blur", "input"] }],
  password: [{ required: true, message: "Password is required", trigger: ["blur", "input"] }]
};

const bootstrapRules: FormRules = {
  name: [{ required: true, message: "Name is required", trigger: ["blur", "input"] }],
  username: [{ required: true, message: "Username is required", trigger: ["blur", "input"] }],
  password: [
    { required: true, message: "Password is required", trigger: ["blur", "input"] },
    { min: 6, message: "At least 6 characters", trigger: ["blur", "input"] }
  ]
};

function redirectTarget(): string {
  return typeof route.query.redirect === "string" ? route.query.redirect : "/";
}

async function submitLogin(): Promise<void> {
  try {
    await formRef.value?.validate();
  } catch {
    return;
  }
  errorMessage.value = "";
  submitting.value = true;
  try {
    await authStore.login(model.username, model.password);
    await router.push(redirectTarget());
  } catch (error) {
    errorMessage.value = apiErrorMessage(error, "Unable to sign in");
  } finally {
    submitting.value = false;
  }
}

async function submitBootstrap(): Promise<void> {
  try {
    await formRef.value?.validate();
  } catch {
    return;
  }
  errorMessage.value = "";
  submitting.value = true;
  try {
    await createUser({
      name: model.name,
      username: model.username,
      password: model.password,
      is_admin: true,
      active: true,
      all_apps: true
    });
    await apiLogin(model.username, model.password);
    await authStore.initialize();
    await router.push("/");
  } catch (error) {
    errorMessage.value = apiErrorMessage(error, "Unable to create the first administrator");
  } finally {
    submitting.value = false;
  }
}

onMounted(async () => {
  try {
    const status = await getBootstrapStatus();
    bootstrapAvailable.value = status.can_create_first_admin;
  } catch {
    bootstrapAvailable.value = false;
  } finally {
    statusLoading.value = false;
  }
});
</script>

<template>
  <main class="login">
    <div class="login__glow" aria-hidden="true" />
    <NCard class="login__card">
      <div class="login__brand">
        <BrandLogo :size="36" :show-text="false" />
        <div>
          <div class="login__title">Ruto Admin</div>
          <div class="login__subtitle muted">
            {{ bootstrapAvailable ? "Create the first administrator" : "API gateway control plane" }}
          </div>
        </div>
      </div>

      <div v-if="statusLoading" class="login__loading">
        <NSpin size="medium" />
      </div>

      <NForm
        v-else-if="bootstrapAvailable"
        ref="formRef"
        :model="model"
        :rules="bootstrapRules"
        @submit.prevent="submitBootstrap"
      >
        <NFormItem label="Name" path="name">
          <NInput v-model:value="model.name" placeholder="Jane Doe" />
        </NFormItem>
        <NFormItem label="Username" path="username">
          <NInput v-model:value="model.username" placeholder="admin" />
        </NFormItem>
        <NFormItem label="Password" path="password">
          <NInput
            v-model:value="model.password"
            type="password"
            show-password-on="click"
            placeholder="Choose a strong password"
            @keyup.enter="submitBootstrap"
          />
        </NFormItem>
        <NButton type="primary" block :loading="submitting" @click="submitBootstrap">
          Create administrator
        </NButton>
      </NForm>

      <NForm v-else ref="formRef" :model="model" :rules="loginRules" @submit.prevent="submitLogin">
        <NFormItem label="Username" path="username">
          <NInput v-model:value="model.username" placeholder="admin" />
        </NFormItem>
        <NFormItem label="Password" path="password">
          <NInput
            v-model:value="model.password"
            type="password"
            show-password-on="click"
            placeholder="Your password"
            @keyup.enter="submitLogin"
          />
        </NFormItem>
        <NButton type="primary" block :loading="submitting" @click="submitLogin">Sign in</NButton>
        <p class="login__hint muted">
          On token expiry the session is renewed once from saved credentials.
        </p>
      </NForm>

      <NAlert v-if="errorMessage" class="login__alert" type="error" :bordered="false">
        {{ errorMessage }}
      </NAlert>
    </NCard>
  </main>
</template>

<style scoped>
.login {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  padding: 24px;
  overflow: hidden;
}

.login__glow {
  position: absolute;
  top: -20%;
  left: 50%;
  width: 640px;
  height: 640px;
  transform: translateX(-50%);
  background: radial-gradient(
    circle,
    rgba(91, 130, 240, 0.18),
    rgba(34, 211, 197, 0.08) 45%,
    transparent 70%
  );
  pointer-events: none;
}

.login__card {
  position: relative;
  width: 100%;
  max-width: 400px;
  box-shadow: var(--shadow-lg);
}

.login__brand {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 22px;
}

.login__title {
  font-size: 19px;
  font-weight: 700;
}

.login__subtitle {
  font-size: 13px;
}

.login__loading {
  display: flex;
  justify-content: center;
  padding: 32px 0;
}

.login__hint {
  margin: 12px 0 0;
  font-size: 12px;
  text-align: center;
}

.login__alert {
  margin-top: 16px;
}
</style>
