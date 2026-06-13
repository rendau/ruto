import { emptyAuth, emptyLogging } from "@/api/normalize";
import type { AppMain, EndpointMain } from "@/api/types";

export function emptyApp(): AppMain {
  return {
    id: "",
    active: true,
    exclude_from_metrics: false,
    path_prefix: "",
    name: "",
    backend: { url: "", swagger_url: "", grpc_url: "", headers: {}, query_params: {} },
    auth: emptyAuth(),
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
    auth: emptyAuth(),
    logging: emptyLogging(),
    variables: []
  };
}
