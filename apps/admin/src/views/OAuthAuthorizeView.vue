<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute } from "vue-router";
import { NAlert, NButton, NCard, NIcon, NSpin } from "naive-ui";
import { EyeOffOutline, GitPullRequestOutline } from "@vicons/ionicons5";
import { storeToRefs } from "pinia";
import { useAuthStore } from "@/stores/auth";
import { authorize, describeAuthorization } from "@/api/oauth";
import { apiErrorMessage } from "@/api/http";
import BrandLogo from "@/components/common/BrandLogo.vue";
import type { OAuthAuthorizeInfoRep, OAuthAuthorizeReq } from "@/api/types";

const route = useRoute();
const { profile } = storeToRefs(useAuthStore());

function queryParam(name: string): string {
  const value = route.query[name];
  return typeof value === "string" ? value : "";
}

// The authorization request is passed to the backend as is; it validates
// the client and the redirect before anything is shown or redirected.
const request = computed<OAuthAuthorizeReq>(() => ({
  response_type: queryParam("response_type"),
  client_id: queryParam("client_id"),
  redirect_uri: queryParam("redirect_uri"),
  state: queryParam("state"),
  code_challenge: queryParam("code_challenge"),
  code_challenge_method: queryParam("code_challenge_method"),
  scope: queryParam("scope"),
  resource: queryParam("resource")
}));

const info = ref<OAuthAuthorizeInfoRep | null>(null);
const loading = ref(true);
const errorMessage = ref("");
const deciding = ref<"" | "allow" | "deny">("");
const redirecting = ref(false);

async function decide(approve: boolean): Promise<void> {
  deciding.value = approve ? "allow" : "deny";
  errorMessage.value = "";
  try {
    const rep = await authorize(request.value, approve);
    redirecting.value = true;
    window.location.assign(rep.redirect_url);
  } catch (error) {
    errorMessage.value = apiErrorMessage(error, "Unable to complete the authorization");
    deciding.value = "";
  }
}

onMounted(async () => {
  try {
    info.value = await describeAuthorization(request.value);
  } catch (error) {
    errorMessage.value = apiErrorMessage(error, "Invalid authorization request");
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <main class="consent">
    <div class="consent__glow" aria-hidden="true" />
    <NCard class="consent__card">
      <div class="consent__brand">
        <BrandLogo :size="34" />
        <div class="consent__subtitle muted">Authorize access to the MCP server</div>
      </div>

      <div v-if="loading || redirecting" class="consent__loading">
        <NSpin size="medium" />
        <span v-if="redirecting" class="muted">Returning to {{ info?.redirect_host }}…</span>
      </div>

      <template v-else-if="info">
        <p class="consent__lead">
          <strong>{{ info.client_name }}</strong> wants to connect to ruto as
          <strong>{{ profile?.username }}</strong>.
        </p>

        <ul class="consent__scopes">
          <li>
            <NIcon :component="EyeOffOutline" class="consent__scope-icon" />
            <span>Read the gateway configuration. Secrets stay hidden.</span>
          </li>
          <li>
            <NIcon :component="GitPullRequestOutline" class="consent__scope-icon" />
            <span>
              Propose changes. Each one waits for an administrator under Change requests.
            </span>
          </li>
        </ul>

        <p class="consent__redirect muted">
          After your decision you return to <code>{{ info.redirect_host }}</code>. Allow only if you
          started connecting this app yourself. You can disconnect it any time in Profile.
        </p>

        <div class="consent__actions">
          <NButton :disabled="Boolean(deciding)" :loading="deciding === 'deny'" @click="decide(false)">
            Deny
          </NButton>
          <NButton
            type="primary"
            :disabled="Boolean(deciding)"
            :loading="deciding === 'allow'"
            @click="decide(true)"
          >
            Allow
          </NButton>
        </div>
      </template>

      <NAlert v-if="errorMessage" class="consent__alert" type="error" :bordered="false">
        {{ errorMessage }}
      </NAlert>

      <RouterLink v-if="!loading && !info" :to="{ name: 'dashboard' }" class="consent__home">
        Go to the dashboard
      </RouterLink>
    </NCard>
  </main>
</template>

<style scoped>
.consent {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  padding: 24px 16px;
  overflow: hidden;
}

.consent__glow {
  position: absolute;
  top: -20%;
  left: 50%;
  width: 640px;
  height: 640px;
  transform: translateX(-50%);
  background: radial-gradient(
    circle,
    rgba(99, 226, 183, 0.16),
    rgba(99, 226, 183, 0.05) 45%,
    transparent 70%
  );
  pointer-events: none;
}

.consent__card {
  position: relative;
  width: 100%;
  max-width: 440px;
  box-shadow: var(--shadow-lg);
}

.consent__brand {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  margin-bottom: 22px;
}

.consent__subtitle {
  font-size: 13px;
}

.consent__loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 32px 0;
  font-size: 13px;
}

.consent__lead {
  margin: 0 0 16px;
  font-size: 15px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.consent__scopes {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin: 0 0 16px;
  padding: 12px 14px;
  list-style: none;
  border: 1px solid var(--c-border);
  border-radius: 10px;
  font-size: 13.5px;
}

.consent__scopes li {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.consent__scope-icon {
  flex-shrink: 0;
  margin-top: 2px;
  color: var(--c-text-3);
}

.consent__redirect {
  margin: 0 0 20px;
  font-size: 12.5px;
  line-height: 1.5;
}

.consent__redirect code {
  font-family: var(--font-mono);
  font-size: 12px;
}

.consent__actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.consent__alert {
  margin-top: 16px;
}

.consent__home {
  display: inline-block;
  margin-top: 14px;
  font-size: 13px;
  color: var(--c-primary);
}
</style>
