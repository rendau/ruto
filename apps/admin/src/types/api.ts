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

export interface AppBackend {
  url: string;
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

export interface EndpointBackend {
  custom_path: string;
}

export interface EndpointMain {
  id: string;
  app_id: string;
  active: boolean;
  method: string;
  path: string;
  backend: EndpointBackend;
  auth: Auth;
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

export interface UsrLoginRep {
  jwt: string;
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
