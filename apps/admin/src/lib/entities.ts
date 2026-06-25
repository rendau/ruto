import { emptyLogging, emptyTransform } from "@/api/normalize";
import type { AppMain, Auth, EndpointMain } from "@/api/types";

// New entities default auth to "enabled + extend" with no methods — i.e. inherit
// the parent's auth and add nothing. This is the baseline treated as "default".
export function defaultAuth(): Auth {
  return { enabled: true, mode: "extend", methods: [] };
}

export function emptyApp(): AppMain {
  return {
    id: "",
    active: true,
    exclude_from_metrics: false,
    path_prefix: "",
    name: "",
    backend: { url: "", swagger_url: "", grpc_url: "", headers: {}, query_params: {} },
    auth: defaultAuth(),
    logging: emptyLogging(),
    variables: []
  };
}

export function emptyEndpoint(appId: string): EndpointMain {
  return {
    id: "",
    app_id: appId,
    active: true,
    exclude_from_metrics: false,
    type: "http",
    http: { method: "GET", path: "" },
    grpc: { service: "", method: "", path: "" },
    backend: { custom_path: "", headers: {}, query_params: {} },
    auth: defaultAuth(),
    logging: emptyLogging(),
    variables: [],
    transform: emptyTransform()
  };
}
