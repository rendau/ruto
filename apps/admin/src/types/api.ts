export interface ErrorRep {
  code: string;
  message: string;
  fields?: Record<string, string>;
}

export interface PaginationInfoSt {
  page: number;
  page_size: number;
  total_count: number;
}

export interface ListParamsSt {
  page?: number;
  page_size?: number;
  with_total_count?: boolean;
  only_count?: boolean;
  sort_name?: string;
  sort?: string[];
}

export interface Auth {
  enabled: boolean;
  mode: string;
  methods: AuthMethod[];
}

export interface AuthMethod {
  basic?: AuthMethodBasic;
  api_key?: AuthMethodApiKey;
  jwt?: AuthMethodJwt;
  ip_validation?: AuthMethodIpValidation;
}

export interface AuthMethodBasic {
  users: AuthMethodBasicUser[];
}

export interface AuthMethodBasicUser {
  username: string;
  password: string;
}

export interface AuthMethodApiKey {
  header: string;
  keys: string[];
}

export interface AuthMethodJwt {
  kid: string;
  roles: string[];
}

export interface AuthMethodIpValidation {
  allowed_ips: string[];
}

export interface RootCors {
  enabled: boolean;
  allow_credentials: boolean;
  max_age: string;
  allow_origins: string[];
  allow_methods: string[];
  allow_headers: string[];
}

export interface RootJwt {
  jwk_url: string;
}

export interface RootMain {
  base_url: string;
  cors: RootCors;
  jwt: RootJwt[];
  auth: Auth;
}

export interface RootJwtKidsRep {
  kids: string[];
}

export interface RootJwtKidsReq {
  urls: string[];
}

export interface AppBackend {
  url: string;
  swagger_url: string;
  grpc_port: number;
}

export interface AppMain {
  id: string;
  active: boolean;
  path_prefix: string;
  name: string;
  backend: AppBackend;
  auth: Auth;
}

export interface AppListRep {
  pagination_info: PaginationInfoSt;
  results: AppMain[];
}

export interface AppCreateRep {
  id: string;
}

export interface AppSwaggerEndpoint {
  method: string;
  path: string;
}

export interface AppSwaggerEndpointsDiffRep {
  unregistered: AppSwaggerEndpoint[];
  registered_invalid: AppSwaggerEndpoint[];
}

export interface AppGrpcReflectionEndpoint {
  service: string;
  method: string;
  path: string;
}

export interface AppGrpcReflectionEndpointsRep {
  results: AppGrpcReflectionEndpoint[];
}

export interface AppGetSwaggerUrlByBackendUrlReq {
  backend_url: string;
}

export interface AppGetSwaggerUrlByBackendUrlRep {
  swagger_url: string;
}

export interface EndpointBackend {
  custom_path: string;
}

export type EndpointType = "http" | "grpc";

export interface EndpointGrpc {
  service: string;
  method: string;
  path: string;
}

export interface EndpointMain {
  id: string;
  app_id: string;
  active: boolean;
  method: string;
  path: string;
  backend: EndpointBackend;
  auth: Auth;
  type: EndpointType;
  grpc: EndpointGrpc;
}

export interface EndpointListRep {
  pagination_info: PaginationInfoSt;
  results: EndpointMain[];
}

export interface EndpointCreateRep {
  id: string;
}

export interface UsrMain {
  id: number;
  active: boolean;
  is_admin: boolean;
  name: string;
  username: string;
  password: string;
}

export interface UsrListRep {
  pagination_info: PaginationInfoSt;
  results: UsrMain[];
}

export interface UsrCreateRep {
  id: number;
}

export interface UsrCreateReq {
  active?: boolean;
  is_admin?: boolean;
  name?: string;
  username?: string;
  password?: string;
}

export interface UsrEditReq {
  id: number;
  active?: boolean;
  is_admin?: boolean;
  name?: string;
  username?: string;
  password?: string;
}

export interface UsrLoginRep {
  jwt: string;
}

export interface UsrBootstrapStatusRep {
  can_create_first_admin: boolean;
}

export interface StatsMethodStats {
  method: string;
  total: number;
  active: number;
}

export interface StatsResponse {
  apps_total: number;
  apps_active: number;
  apps_inactive: number;
  endpoints_total: number;
  endpoints_active: number;
  endpoints_inactive: number;
  users_total: number;
  users_active: number;
  users_admin: number;
  root_jwt_providers: number;
  root_auth_enabled: boolean;
  root_cors_enabled: boolean;
  methods: StatsMethodStats[];
}

export interface GatewayStateItem {
  gateway_id: string;
  host_name: string;
  snapshot_version: string;
  last_apply_at_unix: number;
  started_at_unix: number;
  last_error: string;
  last_seen_at_unix: number;
  memory_alloc_bytes: number;
  goroutines_count: number;
  status: "online" | "stale" | "offline";
}

export interface GatewayStateListRep {
  results: GatewayStateItem[];
}

export interface SnapshotVersionRep {
  version: string;
}
