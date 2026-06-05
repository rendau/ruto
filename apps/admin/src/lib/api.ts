import type {
  AppCreateRep,
  AppGrpcReflectionEndpointsRep,
  AppListRep,
  AppMain,
  AppGetSwaggerUrlByBackendUrlRep,
  AppGetSwaggerUrlByBackendUrlReq,
  AppSwaggerEndpointsDiffRep,
  EndpointCreateRep,
  EndpointListRep,
  EndpointMain,
  ErrorRep,
  GatewayStateListRep,
  UsrCreateRep,
  UsrCreateReq,
  UsrEditReq,
  UsrListRep,
  SnapshotVersionRep,
  StatsResponse,
  RootMain,
  RootJwtKidsReq,
  RootJwtKidsRep,
  RootInterpolateReq,
  UsrLoginRep,
  UsrMain,
  UsrBootstrapStatusRep,
  AppInterpolateReq,
  AppInheritedReq,
  EndpointInterpolateReq,
  EndpointInheritedReq,
  Variable
} from "../types/api";
import { API_BASE_URL } from "./config";
import { clearSession, getToken, renewTokenOnce, setCredentials, setToken } from "./auth-session";

export class ApiError extends Error {
  code: string;
  status: number;
  fields: Record<string, string>;

  constructor(message: string, code: string, status: number, fields?: Record<string, string>) {
    super(message);
    this.code = code;
    this.status = status;
    this.fields = fields || {};
  }
}

function isAuthError(error: ApiError): boolean {
  return error.status === 401 || error.code === "not_authorized";
}

function notifyAuthRequired(): void {
  window.dispatchEvent(new CustomEvent("auth:required"));
}

function withQuery(path: string, query: Record<string, string | number | boolean | undefined | string[]>): string {
  const params = new URLSearchParams();

  for (const [key, value] of Object.entries(query)) {
    if (value === undefined) {
      continue;
    }
    if (Array.isArray(value)) {
      for (const item of value) {
        params.append(key, item);
      }
      continue;
    }
    params.set(key, String(value));
  }

  const queryStr = params.toString();
  return queryStr ? `${path}?${queryStr}` : path;
}

function variablesToArray(variables?: Variable[] | Record<string, string> | null): Variable[] {
  if (!variables) {
    return [];
  }
  if (Array.isArray(variables)) {
    return variables.map((item) => ({
      key: item?.key || "",
      value: item?.value || ""
    }));
  }
  return Object.entries(variables).map(([key, value]) => ({
    key,
    value: value || ""
  }));
}

function variablesToMap(variables?: Variable[] | Record<string, string> | null): Record<string, string> {
  const result: Record<string, string> = {};
  for (const item of variablesToArray(variables)) {
    const key = (item.key || "").trim();
    if (!key) {
      continue;
    }
    result[key] = item.value || "";
  }
  return result;
}

function normalizeRoot(value: RootMain): RootMain {
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
    auth: value?.auth || {
      enabled: false,
      mode: "extend",
      methods: []
    },
    variables: variablesToArray((value as RootMain & { variables?: Variable[] | Record<string, string> }).variables)
  };
}

function normalizeApp(value: AppMain): AppMain {
  return {
    id: value?.id || "",
    active: Boolean(value?.active),
    path_prefix: value?.path_prefix || "",
    name: value?.name || "",
    backend: {
      url: value?.backend?.url || "",
      swagger_url: value?.backend?.swagger_url || "",
      grpc_url: value?.backend?.grpc_url || "",
      headers: value?.backend?.headers || {},
      query_params: value?.backend?.query_params || {}
    },
    auth: value?.auth || {
      enabled: false,
      mode: "extend",
      methods: []
    },
    variables: variablesToArray((value as AppMain & { variables?: Variable[] | Record<string, string> }).variables)
  };
}

function normalizeEndpoint(value: EndpointMain): EndpointMain {
  const endpointWithLegacyFields = value as EndpointMain & {
    method?: string;
    path?: string;
    http?: { method?: string; path?: string };
  };

  return {
    id: value?.id || "",
    app_id: value?.app_id || "",
    active: Boolean(value?.active),
    http: {
      method: endpointWithLegacyFields.http?.method || endpointWithLegacyFields.method || "",
      path: endpointWithLegacyFields.http?.path || endpointWithLegacyFields.path || ""
    },
    backend: {
      custom_path: value?.backend?.custom_path || "",
      headers: value?.backend?.headers || {},
      query_params: value?.backend?.query_params || {}
    },
    auth: value?.auth || {
      enabled: false,
      mode: "extend",
      methods: []
    },
    type: value?.type === "grpc" ? "grpc" : "http",
    grpc: {
      service: value?.grpc?.service || "",
      method: value?.grpc?.method || "",
      path: value?.grpc?.path || ""
    },
    variables: variablesToArray((value as EndpointMain & { variables?: Variable[] | Record<string, string> }).variables)
  };
}

function serializeRoot(value: RootMain): RootMain {
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
    auth: value?.auth || {
      enabled: false,
      mode: "extend",
      methods: []
    },
    variables: variablesToMap(value?.variables) as unknown as Variable[]
  };
}

function serializeApp(value: AppMain): AppMain {
  return {
    ...normalizeApp(value),
    variables: variablesToMap(value?.variables) as unknown as Variable[]
  };
}

function serializeEndpoint(value: EndpointMain): EndpointMain {
  const normalized = normalizeEndpoint(value);
  return {
    ...normalized,
    variables: variablesToMap(value?.variables) as unknown as Variable[]
  };
}

async function parseApiError(response: Response): Promise<ApiError> {
  const payload = (await response.json().catch(() => ({}))) as Partial<ErrorRep>;
  const code = payload.code || "service_not_available";
  const message = payload.message || `Request failed with status ${response.status}`;
  return new ApiError(message, code, response.status, payload.fields);
}

interface FetchOptions {
  retryOnAuth?: boolean;
}

async function apiFetch<T>(path: string, init?: RequestInit, options: FetchOptions = {}): Promise<T> {
  const headers = new Headers(init?.headers || {});
  headers.set("Content-Type", "application/json");

  const token = getToken();
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers
  });

  if (response.ok) {
    if (response.status === 204) {
      return undefined as T;
    }
    const text = await response.text();
    return (text ? JSON.parse(text) : {}) as T;
  }

  const parsedError = await parseApiError(response);
  const canRetry = options.retryOnAuth !== false && isAuthError(parsedError);
  if (!canRetry) {
    throw parsedError;
  }

  const renewed = await renewTokenOnce();
  if (!renewed) {
    clearSession();
    notifyAuthRequired();
    throw parsedError;
  }

  return apiFetch<T>(path, init, { retryOnAuth: false });
}

export async function login(username: string, password: string): Promise<void> {
  const response = await fetch(`${API_BASE_URL}/usr/login`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({ username, password })
  });
  const payload = (await response.json().catch(() => ({}))) as Partial<UsrLoginRep & ErrorRep>;
  if (!response.ok || !payload.jwt) {
    throw new ApiError(payload.message || "Login failed", payload.code || "not_authorized", response.status);
  }
  setToken(payload.jwt);
  setCredentials(username, password);
}

export function logout(): void {
  clearSession();
}

export function getProfile(): Promise<UsrMain> {
  return apiFetch<UsrMain>("/usr/profile");
}

export function getBootstrapStatus(): Promise<UsrBootstrapStatusRep> {
  return apiFetch<UsrBootstrapStatusRep>("/usr/bootstrap/status");
}

export function updateProfile(req: { name?: string; password?: string }): Promise<void> {
  return apiFetch<void>("/usr/profile", {
    method: "PUT",
    body: JSON.stringify(req)
  });
}

export function listUsers(req?: { search?: string; page?: number; page_size?: number; with_total_count?: boolean }): Promise<UsrListRep> {
  return apiFetch<UsrListRep>(
    withQuery("/usr", {
      "list_params.page": req?.page,
      "list_params.page_size": req?.page_size,
      "list_params.with_total_count": req?.with_total_count,
      search: req?.search
    })
  );
}

export function createUser(req: UsrCreateReq): Promise<UsrCreateRep> {
  return apiFetch<UsrCreateRep>("/usr", {
    method: "POST",
    body: JSON.stringify(req)
  });
}

export function getUser(id: number): Promise<UsrMain> {
  return apiFetch<UsrMain>(`/usr/${id}`);
}

export function updateUser(req: UsrEditReq): Promise<void> {
  return apiFetch<void>(`/usr/${req.id}`, {
    method: "PUT",
    body: JSON.stringify(req)
  });
}

export function deleteUser(id: number): Promise<void> {
  return apiFetch<void>(`/usr/${id}`, {
    method: "DELETE"
  });
}

export function listApps(req?: { active?: boolean }): Promise<AppListRep> {
  return apiFetch<AppListRep>(
    withQuery("/app", {
      active: req?.active
    })
  ).then((rep) => ({
    ...rep,
    results: (rep.results || []).map(normalizeApp)
  }));
}

export function getApp(id: string): Promise<AppMain> {
  return apiFetch<AppMain>(`/app/${id}`).then(normalizeApp);
}

export function createApp(req: AppMain): Promise<AppCreateRep> {
  return apiFetch<AppCreateRep>("/app", {
    method: "POST",
    body: JSON.stringify(serializeApp(req))
  });
}

export function updateApp(req: AppMain): Promise<void> {
  return apiFetch<void>(`/app/${req.id}`, {
    method: "PUT",
    body: JSON.stringify(serializeApp(req))
  });
}

export function deleteApp(id: string): Promise<void> {
  return apiFetch<void>(`/app/${id}`, {
    method: "DELETE"
  });
}

export function getAppSwaggerEndpointsDiff(id: string): Promise<AppSwaggerEndpointsDiffRep> {
  return apiFetch<AppSwaggerEndpointsDiffRep>(`/app/${id}/swagger/endpoints-diff`);
}

export function getAppGrpcReflectionEndpoints(id: string): Promise<AppGrpcReflectionEndpointsRep> {
  return apiFetch<AppGrpcReflectionEndpointsRep>(`/app/${id}/grpc/reflection/endpoints`);
}

export function getAppSwaggerUrlByBackendUrl(req: AppGetSwaggerUrlByBackendUrlReq): Promise<AppGetSwaggerUrlByBackendUrlRep> {
  return apiFetch<AppGetSwaggerUrlByBackendUrlRep>(
    withQuery("/app/swagger/url/by-backend-url", {
      backend_url: req.backend_url
    })
  );
}

export function getAppInterpolate(req: AppInterpolateReq): Promise<AppMain> {
  return apiFetch<AppMain>(`/app/${req.id || ""}/interpolate`, {
    method: "POST",
    body: JSON.stringify({
      id: req.id || "",
      variables: variablesToMap(req.variables)
    })
  }).then(normalizeApp);
}

export function getAppInherited(req: AppInheritedReq): Promise<AppMain> {
  return apiFetch<AppMain>(`/app/${req.id || ""}/inherited`, {
    method: "POST",
    body: JSON.stringify({
      id: req.id || "",
      variables: variablesToMap(req.variables)
    })
  }).then(normalizeApp);
}

export function listEndpoints(req?: { app_id?: string; active?: boolean }): Promise<EndpointListRep> {
  return apiFetch<EndpointListRep>(
    withQuery("/endpoint", {
      app_id: req?.app_id,
      active: req?.active
    })
  ).then((rep) => ({
    ...rep,
    results: (rep.results || []).map(normalizeEndpoint)
  }));
}

export function getEndpoint(id: string): Promise<EndpointMain> {
  return apiFetch<EndpointMain>(`/endpoint/${id}`).then(normalizeEndpoint);
}

export function createEndpoint(req: EndpointMain): Promise<EndpointCreateRep> {
  return apiFetch<EndpointCreateRep>("/endpoint", {
    method: "POST",
    body: JSON.stringify(serializeEndpoint(req))
  });
}

export function updateEndpoint(req: EndpointMain): Promise<void> {
  return apiFetch<void>(`/endpoint/${req.id}`, {
    method: "PUT",
    body: JSON.stringify(serializeEndpoint(req))
  });
}

export function deleteEndpoint(id: string): Promise<void> {
  return apiFetch<void>(`/endpoint/${id}`, {
    method: "DELETE"
  });
}

export function getEndpointInterpolate(req: EndpointInterpolateReq): Promise<EndpointMain> {
  return apiFetch<EndpointMain>(`/endpoint/${req.id || ""}/interpolate`, {
    method: "POST",
    body: JSON.stringify({
      id: req.id || "",
      variables: variablesToMap(req.variables)
    })
  }).then(normalizeEndpoint);
}

export function getEndpointInherited(req: EndpointInheritedReq): Promise<EndpointMain> {
  return apiFetch<EndpointMain>(`/endpoint/${req.id || ""}/inherited`, {
    method: "POST",
    body: JSON.stringify({
      id: req.id || "",
      variables: variablesToMap(req.variables)
    })
  }).then(normalizeEndpoint);
}

export function getRoot(): Promise<RootMain> {
  return apiFetch<RootMain>("/root").then(normalizeRoot);
}

export function setRoot(req: RootMain): Promise<void> {
  return apiFetch<void>("/root", {
    method: "POST",
    body: JSON.stringify(serializeRoot(req))
  });
}

export function getRootJwtKidsByUrls(req: RootJwtKidsReq): Promise<RootJwtKidsRep> {
  return apiFetch<RootJwtKidsRep>(
    withQuery("/root/jwt/kids/by-urls", {
      urls: req.urls || []
    })
  );
}

export function getRootInterpolate(req: RootInterpolateReq): Promise<RootMain> {
  return apiFetch<RootMain>("/root/interpolate", {
    method: "POST",
    body: JSON.stringify({
      variables: variablesToMap(req.variables)
    })
  }).then(normalizeRoot);
}

export function getStats(): Promise<StatsResponse> {
  return apiFetch<StatsResponse>("/stats");
}

export function deploySnapshot(): Promise<void> {
  return apiFetch<void>("/snapshot/deploy", {
    method: "POST"
  });
}

export function getSnapshotVersion(): Promise<SnapshotVersionRep> {
  return apiFetch<SnapshotVersionRep>("/snapshot/version");
}

export function listGateways(): Promise<GatewayStateListRep> {
  return apiFetch<GatewayStateListRep>("/gateway");
}
