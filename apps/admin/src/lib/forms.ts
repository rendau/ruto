import type {
  Auth,
  AuthMethod,
  AuthMethodApiKey,
  AuthMethodBasic,
  AuthMethodBasicUser,
  AuthMethodIpValidation,
  AuthMethodJwt
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
    result.api_key = { header: "", keys: [] };
  } else if (type === "jwt") {
    result.jwt = { kid: "", roles: [] };
  } else {
    result.ip_validation = { allowed_ips: [] };
  }
  return result;
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
    keys: [...(value.keys || [])]
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
    allowed_ips: [...(value.allowed_ips || [])]
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
