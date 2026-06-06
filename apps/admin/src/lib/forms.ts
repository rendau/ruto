import type {
  Auth,
  AuthMethod,
  AuthMethodApiKey,
  AuthMethodApiKeyItem,
  AuthMethodBasic,
  AuthMethodBasicUser,
  AuthMethodIpValidation,
  AuthMethodIpValidationItem,
  AuthMethodJwt,
  Variable
} from "../types/api";

export const emptyAuth: Auth = {
  enabled: false,
  mode: "extend",
  methods: []
};

export function createEmptyAuthMethod(type: "basic" | "api_key" | "jwt" | "ip_validation"): AuthMethod {
  const result: AuthMethod = {};
  if (type === "basic") {
    result.basic = { users: [{ username: "", password: "" }] };
  } else if (type === "api_key") {
    result.api_key = { header: "", keys: [{ name: "", key: "" }] };
  } else if (type === "jwt") {
    result.jwt = { kid: "", roles: [] };
  } else {
    result.ip_validation = { allowed_ips: [{ name: "", ip: "" }] };
  }
  return result;
}

function normalizeApiKeyItem(value: AuthMethodApiKeyItem): AuthMethodApiKeyItem {
  return { name: value?.name || "", key: value?.key || "" };
}

function normalizeIpItem(value: AuthMethodIpValidationItem): AuthMethodIpValidationItem {
  return { name: value?.name || "", ip: value?.ip || "" };
}

function cloneBasicUser(value: AuthMethodBasicUser): AuthMethodBasicUser {
  return {
    username: value.username || "",
    password: value.password || ""
  };
}

function cloneBasic(value?: AuthMethodBasic): AuthMethodBasic | undefined {
  if (!value) {
    return undefined;
  }
  return {
    users: (value.users || []).map(cloneBasicUser)
  };
}

function cloneApiKey(value?: AuthMethodApiKey): AuthMethodApiKey | undefined {
  if (!value) {
    return undefined;
  }
  return {
    header: value.header || "",
    keys: (value.keys || []).map(normalizeApiKeyItem)
  };
}

function cloneJwt(value?: AuthMethodJwt): AuthMethodJwt | undefined {
  if (!value) {
    return undefined;
  }
  return {
    kid: value.kid || "",
    roles: [...(value.roles || [])]
  };
}

function cloneIpValidation(value?: AuthMethodIpValidation): AuthMethodIpValidation | undefined {
  if (!value) {
    return undefined;
  }
  return {
    allowed_ips: (value.allowed_ips || []).map(normalizeIpItem)
  };
}

export function cloneAuthMethod(value?: AuthMethod): AuthMethod {
  return {
    basic: cloneBasic(value?.basic),
    api_key: cloneApiKey(value?.api_key),
    jwt: cloneJwt(value?.jwt),
    ip_validation: cloneIpValidation(value?.ip_validation)
  };
}

export function normalizeAuth(value?: Auth | null): Auth {
  if (!value) {
    return { ...emptyAuth };
  }

  return {
    enabled: !!value.enabled,
    mode: value.mode === "replace" ? "replace" : "extend",
    methods: (value.methods || []).map((method) => cloneAuthMethod(method))
  };
}

export function generateSecret(length = 32): string {
  const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  const bytes = new Uint8Array(length);
  crypto.getRandomValues(bytes);
  let result = "";
  for (let i = 0; i < length; i += 1) {
    result += charset[bytes[i] % charset.length];
  }
  return result;
}

export function prettyJson(value: unknown): string {
  return JSON.stringify(value, null, 2);
}

export function parseAuthFromJson(value: string): Auth {
  if (!value.trim()) {
    return { ...emptyAuth };
  }
  const parsed = JSON.parse(value) as Auth;
  return normalizeAuth(parsed);
}

export function linesToArray(value: string): string[] {
  return value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

export function arrayToLines(value?: string[]): string {
  return (value || []).join("\n");
}

export function recordToKeyValueLines(value?: Record<string, string> | null): string {
  if (!value) {
    return "";
  }
  return Object.entries(value)
    .map(([key, itemValue]) => `${key}: ${itemValue}`)
    .join("\n");
}

export function keyValueLinesToRecord(value: string): Record<string, string> {
  const result: Record<string, string> = {};
  for (const line of value.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) {
      continue;
    }
    const colonIndex = trimmed.indexOf(":");
    const equalsIndex = trimmed.indexOf("=");
    let separatorIndex = -1;
    if (colonIndex >= 0 && equalsIndex >= 0) {
      separatorIndex = Math.min(colonIndex, equalsIndex);
    } else {
      separatorIndex = Math.max(colonIndex, equalsIndex);
    }
    if (separatorIndex < 0) {
      result[trimmed] = "";
      continue;
    }
    const key = trimmed.slice(0, separatorIndex).trim();
    if (!key) {
      continue;
    }
    result[key] = trimmed.slice(separatorIndex + 1).trim();
  }
  return result;
}

export function variablesToKeyValueLines(value?: Variable[] | null): string {
  return (value || []).map((item) => `${item.key}: ${item.value}`).join("\n");
}

export function keyValueLinesToVariables(value: string): Variable[] {
  const result: Variable[] = [];
  for (const line of value.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) {
      continue;
    }
    const colonIndex = trimmed.indexOf(":");
    const equalsIndex = trimmed.indexOf("=");
    let separatorIndex = -1;
    if (colonIndex >= 0 && equalsIndex >= 0) {
      separatorIndex = Math.min(colonIndex, equalsIndex);
    } else {
      separatorIndex = Math.max(colonIndex, equalsIndex);
    }
    if (separatorIndex < 0) {
      result.push({ key: trimmed, value: "" });
      continue;
    }
    const key = trimmed.slice(0, separatorIndex).trim();
    if (!key) {
      continue;
    }
    result.push({ key, value: trimmed.slice(separatorIndex + 1).trim() });
  }
  return result;
}

export function normalizeVariables(value?: Variable[] | Record<string, string> | null): Variable[] {
  if (!value) {
    return [];
  }
  if (Array.isArray(value)) {
    return value.map((item) => ({
      key: item?.key || "",
      value: item?.value || ""
    }));
  }
  return Object.entries(value).map(([key, itemValue]) => ({
    key,
    value: itemValue || ""
  }));
}

export function hasDuplicateVariableKeys(value?: Variable[] | null): boolean {
  const seen = new Set<string>();
  for (const item of value || []) {
    const key = (item.key || "").trim();
    if (!key) {
      continue;
    }
    if (seen.has(key)) {
      return true;
    }
    seen.add(key);
  }
  return false;
}
