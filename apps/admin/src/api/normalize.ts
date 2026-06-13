import type {
  AppMain,
  Auth,
  AuthMethod,
  AuthMethodApiKey,
  AuthMethodApiKeyItem,
  AuthMethodBasic,
  AuthMethodBasicUser,
  AuthMethodIpValidation,
  AuthMethodIpValidationItem,
  AuthMethodJwt,
  EndpointMain,
  Logging,
  RootMain,
  Variable
} from "./types";

export function emptyAuth(): Auth {
  return { enabled: false, mode: "extend", methods: [] };
}

export function emptyLogging(): Logging {
  return {
    mode: "extend",
    level: "",
    headers: false,
    query_params: false,
    req_body: false,
    resp_body: false,
    req_body_limit: 0,
    resp_body_limit: 0
  };
}

export function normalizeLogging(value?: Logging | null): Logging {
  if (!value) {
    return emptyLogging();
  }
  const level =
    value.level === "all" || value.level === "error" || value.level === "none" ? value.level : "";
  return {
    mode: value.mode === "replace" ? "replace" : "extend",
    level,
    headers: !!value.headers,
    query_params: !!value.query_params,
    req_body: !!value.req_body,
    resp_body: !!value.resp_body,
    req_body_limit: Math.max(0, Math.trunc(value.req_body_limit || 0)),
    resp_body_limit: Math.max(0, Math.trunc(value.resp_body_limit || 0))
  };
}

function cloneBasic(value?: AuthMethodBasic): AuthMethodBasic | undefined {
  if (!value) return undefined;
  return {
    users: (value.users || []).map(
      (user: AuthMethodBasicUser): AuthMethodBasicUser => ({
        username: user.username || "",
        password: user.password || ""
      })
    )
  };
}

function cloneApiKey(value?: AuthMethodApiKey): AuthMethodApiKey | undefined {
  if (!value) return undefined;
  return {
    header: value.header || "",
    keys: (value.keys || []).map(
      (item: AuthMethodApiKeyItem): AuthMethodApiKeyItem => ({
        name: item.name || "",
        key: item.key || ""
      })
    )
  };
}

function cloneJwt(value?: AuthMethodJwt): AuthMethodJwt | undefined {
  if (!value) return undefined;
  return { kid: value.kid || "", roles: [...(value.roles || [])] };
}

function cloneIpValidation(value?: AuthMethodIpValidation): AuthMethodIpValidation | undefined {
  if (!value) return undefined;
  return {
    allowed_ips: (value.allowed_ips || []).map(
      (item: AuthMethodIpValidationItem): AuthMethodIpValidationItem => ({
        name: item.name || "",
        ip: item.ip || ""
      })
    )
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
    return emptyAuth();
  }
  return {
    enabled: !!value.enabled,
    mode: value.mode === "replace" ? "replace" : "extend",
    methods: (value.methods || []).map((method) => cloneAuthMethod(method))
  };
}

type RawVariables = Variable[] | Record<string, string> | null | undefined;

export function variablesToArray(variables?: RawVariables): Variable[] {
  if (!variables) {
    return [];
  }
  if (Array.isArray(variables)) {
    return variables.map((item) => ({ key: item?.key || "", value: item?.value || "" }));
  }
  return Object.entries(variables).map(([key, value]) => ({ key, value: value || "" }));
}

export function variablesToMap(variables?: RawVariables): Record<string, string> {
  const result: Record<string, string> = {};
  for (const item of variablesToArray(variables)) {
    const key = (item.key || "").trim();
    if (!key) continue;
    result[key] = item.value || "";
  }
  return result;
}

// The backend serialises variables as an object map but the form layer works
// with an ordered array; these casts bridge the two representations.
type WithRawVariables<T> = Omit<T, "variables"> & { variables?: RawVariables };

export function normalizeRoot(value: RootMain): RootMain {
  const raw = value as WithRawVariables<RootMain>;
  return {
    base_url: value?.base_url || "",
    cors: value?.cors || {
      enabled: false,
      allow_credentials: false,
      max_age: "",
      allow_origins: [],
      allow_methods: [],
      allow_headers: []
    },
    jwt: value?.jwt || [],
    auth: normalizeAuth(value?.auth),
    logging: normalizeLogging(value?.logging),
    log_own_response_errors: Boolean(value?.log_own_response_errors),
    variables: variablesToArray(raw.variables)
  };
}

export function normalizeApp(value: AppMain): AppMain {
  const raw = value as WithRawVariables<AppMain>;
  return {
    id: value?.id || "",
    active: Boolean(value?.active),
    exclude_from_metrics: Boolean(value?.exclude_from_metrics),
    path_prefix: value?.path_prefix || "",
    name: value?.name || "",
    backend: {
      url: value?.backend?.url || "",
      swagger_url: value?.backend?.swagger_url || "",
      grpc_url: value?.backend?.grpc_url || "",
      headers: value?.backend?.headers || {},
      query_params: value?.backend?.query_params || {}
    },
    auth: normalizeAuth(value?.auth),
    logging: normalizeLogging(value?.logging),
    variables: variablesToArray(raw.variables)
  };
}

export function normalizeEndpoint(value: EndpointMain): EndpointMain {
  const raw = value as WithRawVariables<EndpointMain> & {
    method?: string;
    path?: string;
    http?: { method?: string; path?: string };
  };
  return {
    id: value?.id || "",
    app_id: value?.app_id || "",
    active: Boolean(value?.active),
    exclude_from_metrics: Boolean(value?.exclude_from_metrics),
    type: value?.type === "grpc" ? "grpc" : "http",
    http: {
      method: raw.http?.method || raw.method || "",
      path: raw.http?.path || raw.path || ""
    },
    grpc: {
      service: value?.grpc?.service || "",
      method: value?.grpc?.method || "",
      path: value?.grpc?.path || ""
    },
    backend: {
      custom_path: value?.backend?.custom_path || "",
      headers: value?.backend?.headers || {},
      query_params: value?.backend?.query_params || {}
    },
    auth: normalizeAuth(value?.auth),
    logging: normalizeLogging(value?.logging),
    variables: variablesToArray(raw.variables)
  };
}

export function serializeRoot(value: RootMain): unknown {
  return { ...normalizeRoot(value), variables: variablesToMap(value?.variables) };
}

export function serializeApp(value: AppMain): unknown {
  return { ...normalizeApp(value), variables: variablesToMap(value?.variables) };
}

export function serializeEndpoint(value: EndpointMain): unknown {
  return { ...normalizeEndpoint(value), variables: variablesToMap(value?.variables) };
}
