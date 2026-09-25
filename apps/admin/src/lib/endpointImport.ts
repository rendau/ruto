// Parses endpoints pasted from the clipboard. Supported inputs:
//   - OpenAPI / Swagger documents (JSON or YAML);
//   - Postman collections and HAR archives (JSON);
//   - JSON arrays of strings or objects ({method, path|url} or ruto endpoints);
//   - .proto files (service/rpc declarations → gRPC endpoints);
//   - curl commands (multi-line with "\" continuations);
//   - free text, one endpoint per line: "GET /users", "/users", full URLs,
//     .http files, tables (TSV/CSV/markdown), Swagger UI copy (method and path
//     on separate lines), router code (r.Get("/users"), @GetMapping("/users"),
//     mux.HandleFunc("GET /users"), app.get('/users')...), gRPC paths
//     (/pkg.Service/Method).

export type ImportFormat =
  | "openapi"
  | "postman"
  | "har"
  | "json"
  | "proto"
  | "curl"
  | "text";

export const IMPORT_FORMAT_LABELS: Record<ImportFormat, string> = {
  openapi: "OpenAPI",
  postman: "Postman",
  har: "HAR",
  json: "JSON",
  proto: "Proto",
  curl: "curl",
  text: "Text"
};

export interface ParsedHttpEndpoint {
  type: "http";
  // Empty when the source did not name a method: the caller applies its default.
  method: string;
  // Always starts with "/", query and trailing slash stripped.
  path: string;
}

export interface ParsedGrpcEndpoint {
  type: "grpc";
  service: string;
  method: string;
  path: string;
}

export type ParsedEndpoint = ParsedHttpEndpoint | ParsedGrpcEndpoint;

export interface ParseResult {
  formats: ImportFormat[];
  endpoints: ParsedEndpoint[];
  // Non-empty source lines nothing could be extracted from.
  skipped: string[];
}

const METHOD_SET = new Set([
  "GET",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "HEAD",
  "OPTIONS",
  "CONNECT",
  "TRACE"
]);
const WILDCARD_METHODS = new Set(["*", "ANY", "ALL"]);

export function parseEndpointsText(text: string): ParseResult {
  const source = text.replace(/^﻿/, "").trim();
  if (!source) return { formats: [], endpoints: [], skipped: [] };

  return (
    parseJsonDocument(source) ??
    parseYamlOpenApi(source) ??
    parseProto(source) ??
    parseLines(source)
  );
}

// ---- JSON -----------------------------------------------------------------

function parseJsonDocument(source: string): ParseResult | null {
  if (!/^[[{]/.test(source)) return null;
  let doc: unknown;
  try {
    doc = JSON.parse(source);
  } catch {
    return null;
  }

  if (isRecord(doc) && isRecord(doc.paths)) {
    return result(["openapi"], fromOpenApiPaths(doc.paths));
  }
  if (isRecord(doc) && Array.isArray(doc.item)) {
    return result(["postman"], fromPostmanItems(doc.item));
  }
  if (isRecord(doc) && isRecord(doc.log) && Array.isArray(doc.log.entries)) {
    const endpoints = doc.log.entries.flatMap((entry) =>
      isRecord(entry) && isRecord(entry.request)
        ? httpFrom(str(entry.request.method), str(entry.request.url))
        : []
    );
    return result(["har"], endpoints);
  }

  const items = Array.isArray(doc) ? doc : [doc];
  const endpoints: ParsedEndpoint[] = [];
  const skipped: string[] = [];
  for (const item of items) {
    const parsed = typeof item === "string" ? parseLines(item).endpoints : fromJsonObject(item);
    if (parsed.length) {
      endpoints.push(...parsed);
    } else {
      skipped.push(typeof item === "string" ? item : JSON.stringify(item));
    }
  }
  if (!endpoints.length) return null;
  return { formats: ["json"], endpoints, skipped };
}

function fromOpenApiPaths(paths: Record<string, unknown>): ParsedEndpoint[] {
  return Object.entries(paths).flatMap(([path, operations]) =>
    isRecord(operations)
      ? Object.keys(operations)
          .filter((method) => METHOD_SET.has(method.toUpperCase()))
          .flatMap((method) => httpFrom(method, path))
      : []
  );
}

function fromPostmanItems(items: unknown[]): ParsedEndpoint[] {
  return items.flatMap((item): ParsedEndpoint[] => {
    if (!isRecord(item)) return [];
    if (Array.isArray(item.item)) return fromPostmanItems(item.item);
    const request = item.request;
    if (typeof request === "string") return httpFrom("GET", request);
    if (!isRecord(request)) return [];
    const url = request.url;
    const raw = isRecord(url)
      ? str(url.raw) || (Array.isArray(url.path) ? "/" + url.path.map(str).join("/") : "")
      : str(url);
    return httpFrom(str(request.method), raw);
  });
}

function fromJsonObject(item: unknown): ParsedEndpoint[] {
  if (!isRecord(item)) return [];
  // ruto endpoint (as returned by the admin API).
  if (item.type === "grpc" && isRecord(item.grpc)) {
    const grpc = grpcFromPath(str(item.grpc.path));
    return grpc ? [grpc] : [];
  }
  if (isRecord(item.http)) {
    return httpFrom(str(item.http.method), str(item.http.path));
  }
  if (typeof item.service === "string" && typeof item.method === "string") {
    const grpc = grpcFromPath(
      str(item.path) || `/${item.service.replace(/^\/+/, "")}/${item.method}`
    );
    if (grpc) return [grpc];
  }
  const target = str(item.path) || str(item.url) || str(item.route) || str(item.uri);
  if (!target) return [];
  const methods = item.methods ?? item.method ?? item.verb;
  const list = Array.isArray(methods) ? methods.map(str) : [str(methods)];
  return list.flatMap((method) => httpFrom(method, target));
}

// ---- OpenAPI YAML ---------------------------------------------------------

// A tiny indentation-based reader, enough to pull "paths → path → method" out
// of an OpenAPI/Swagger YAML document without a YAML dependency.
function parseYamlOpenApi(source: string): ParseResult | null {
  const lines = source.split(/\r?\n/);
  const pathsLine = lines.findIndex((line) => /^paths:\s*(#.*)?$/.test(line));
  if (pathsLine < 0) return null;

  const endpoints: ParsedEndpoint[] = [];
  let pathIndent = -1;
  let methodIndent = -1;
  let currentPath = "";
  for (const line of lines.slice(pathsLine + 1)) {
    if (!line.trim() || line.trim().startsWith("#")) continue;
    const indent = line.length - line.trimStart().length;
    if (indent === 0) break; // next top-level key
    const key = yamlKey(line.trim());
    if (key === null) continue;

    if (pathIndent < 0 || indent <= pathIndent) {
      pathIndent = indent;
      methodIndent = -1;
      currentPath = key.startsWith("/") ? key : "";
      continue;
    }
    if (!currentPath) continue;
    if (methodIndent < 0) methodIndent = indent;
    if (indent === methodIndent && METHOD_SET.has(key.toUpperCase())) {
      endpoints.push(...httpFrom(key, currentPath));
    }
  }
  if (!endpoints.length) return null;
  return result(["openapi"], endpoints);
}

function yamlKey(line: string): string | null {
  const match = line.match(/^(?:"([^"]+)"|'([^']+)'|([^\s:#][^:#]*?))\s*:(\s|$)/);
  if (!match) return null;
  return (match[1] ?? match[2] ?? match[3] ?? "").trim();
}

// ---- Proto ----------------------------------------------------------------

function parseProto(source: string): ParseResult | null {
  if (!/^\s*service\s+\w+\s*\{/m.test(source) || !/\brpc\s+\w+\s*\(/.test(source)) {
    return null;
  }
  const pkg = source.match(/^\s*package\s+([\w.]+)\s*;/m)?.[1] ?? "";
  const endpoints: ParsedEndpoint[] = [];
  let service = "";
  for (const [, serviceName, rpc] of source.matchAll(
    /\bservice\s+(\w+)\s*\{|\brpc\s+(\w+)\s*\(/g
  )) {
    if (serviceName) {
      service = pkg ? `${pkg}.${serviceName}` : serviceName;
    } else if (service && rpc) {
      endpoints.push({ type: "grpc", service, method: rpc, path: `/${service}/${rpc}` });
    }
  }
  return result(["proto"], endpoints);
}

// ---- Line based: curl, text, code -----------------------------------------

function parseLines(source: string): ParseResult {
  const formats = new Set<ImportFormat>();
  const endpoints: ParsedEndpoint[] = [];
  const skipped: string[] = [];
  // Swagger UI and some tables put the method on its own line, the path below.
  let pendingMethods: string[] = [];

  for (const line of joinContinuations(source)) {
    const trimmed = line.trim();
    if (!trimmed || /^(#|\/\/)/.test(trimmed)) continue;

    if (/^curl\s/i.test(trimmed)) {
      const parsed = parseCurl(trimmed);
      if (parsed) {
        formats.add("curl");
        endpoints.push(parsed);
      } else {
        skipped.push(trimmed);
      }
      pendingMethods = [];
      continue;
    }

    const { methods, targets } = scanLine(trimmed);
    if (!targets.length) {
      if (methods.length) {
        pendingMethods = methods;
      } else {
        skipped.push(trimmed);
      }
      continue;
    }

    const lineMethods = methods.length ? methods : pendingMethods;
    pendingMethods = [];
    formats.add("text");
    for (const target of targets) {
      const grpc = lineMethods.length ? null : grpcFromPath(target);
      if (grpc) {
        endpoints.push(grpc);
        continue;
      }
      for (const method of lineMethods.length ? lineMethods : [""]) {
        endpoints.push(...httpFrom(method, target));
      }
    }
  }

  return { formats: [...formats], endpoints, skipped };
}

function joinContinuations(source: string): string[] {
  const lines: string[] = [];
  let buffer = "";
  for (const line of source.split(/\r?\n/)) {
    const continued = line.match(/^(.*?)\s*[\\^`]\s*$/);
    if (continued && /^\s*curl\s/i.test(buffer + continued[1])) {
      buffer += continued[1] + " ";
      continue;
    }
    lines.push(buffer + line);
    buffer = "";
  }
  if (buffer) lines.push(buffer);
  return lines;
}

function scanLine(line: string): { methods: string[]; targets: string[] } {
  const methods: string[] = [];
  const targets: string[] = [];
  const tokens = line
    .split(/[\s"'`(),;|\[\]]+/)
    .map((token) => token.trim())
    .filter(Boolean);

  for (const token of tokens) {
    const method = methodFromToken(token);
    if (method) {
      if (!methods.includes(method)) methods.push(method);
      continue;
    }
    if (isTarget(token) || isGrpcMethodName(token)) targets.push(token);
  }

  // "GET api/users": a bare word right after an explicit method is a path too.
  if (!targets.length && methods.length && tokens.length === 2) {
    const candidate = tokens[1] ?? "";
    if (/^[\w\-.{}:<>/]+$/.test(candidate) && !methodFromToken(candidate)) targets.push(candidate);
  }
  return { methods, targets };
}

// Recognises plain methods ("GET", "get") and router code: r.Get, router.GET,
// app.get, @app.post, @GetMapping, [HttpGet], HttpMethod.Get, express app.all.
function methodFromToken(token: string): string {
  const upper = token.toUpperCase().replace(/^@|:$/g, "");
  if (METHOD_SET.has(upper)) return upper;
  if (upper === "ANY") return "*";

  const dotted = upper.match(/\.(\w+)$/)?.[1] ?? "";
  if (METHOD_SET.has(dotted)) return dotted;
  if (dotted === "ANY" || dotted === "ALL") return "*";

  const annotated = upper.match(/^(?:HTTP)?(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)(?:MAPPING)?$/);
  return annotated?.[1] ?? "";
}

function isTarget(token: string): boolean {
  if (/^https?:\/\/[^/\s]+/i.test(token)) return true;
  if (/^\{\{[^}]+\}\}\//.test(token)) return true; // Postman / .http variables
  return token.startsWith("/") && !token.startsWith("//");
}

// grpcurl-style "pkg.Service/Method" without the leading slash. The method must be
// capitalised (as in proto) so "example.com/path" is not mistaken for gRPC.
function isGrpcMethodName(token: string): boolean {
  return /^(?:[A-Za-z_]\w*\.)+[A-Za-z_]\w*\/[A-Z]\w*$/.test(token);
}

// ---- curl -----------------------------------------------------------------

// Flags whose next argument is a value, not the URL.
const CURL_VALUE_FLAGS =
  /^(-H|--header|-u|--user|-A|--user-agent|-b|--cookie|-e|--referer|-o|--output|-w|--write-out|-m|--max-time|--connect-timeout|-x|--proxy|--cert|--key|--cacert|-c|--cookie-jar|-r|--range|--resolve)$/;

function parseCurl(command: string): ParsedHttpEndpoint | null {
  const args = shellSplit(command).slice(1);
  let method = "";
  let url = "";
  let hasBody = false;
  let headOnly = false;

  for (let i = 0; i < args.length; i++) {
    const arg = args[i] ?? "";
    const inline = arg.match(/^-X(.+)$/) ?? arg.match(/^--request=(.+)$/);
    if (inline) {
      method = inline[1] ?? "";
    } else if (arg === "-X" || arg === "--request") {
      method = args[++i] ?? "";
    } else if (arg === "--url") {
      url = args[++i] ?? "";
    } else if (/^(-d|--data.*|-F|--form.*|--json|-T|--upload-file)$/.test(arg)) {
      hasBody = true;
      i++;
    } else if (/^-(d|F)./.test(arg)) {
      hasBody = true;
    } else if (arg === "-I" || arg === "--head") {
      headOnly = true;
    } else if (arg === "-G" || arg === "--get") {
      method = method || "GET";
    } else if (CURL_VALUE_FLAGS.test(arg)) {
      i++;
    } else if (!arg.startsWith("-") && !url) {
      url = arg;
    }
  }
  if (!url) return null;
  if (!method) method = headOnly ? "HEAD" : hasBody ? "POST" : "GET";
  return httpFrom(method, url).at(0) ?? null;
}

function shellSplit(command: string): string[] {
  const args: string[] = [];
  const re = /\$'((?:[^'\\]|\\.)*)'|'([^']*)'|"((?:[^"\\]|\\.)*)"|(\S+)/g;
  let match: RegExpExecArray | null;
  while ((match = re.exec(command))) {
    args.push(match[1] ?? match[2] ?? match[3] ?? match[4] ?? "");
  }
  return args;
}

// ---- Normalization --------------------------------------------------------

function httpFrom(method: string, target: string): ParsedHttpEndpoint[] {
  const path = pathFromTarget(target);
  if (path === null) return [];
  const upper = method.trim().toUpperCase();
  const normalizedMethod = WILDCARD_METHODS.has(upper) ? "*" : upper;
  if (normalizedMethod && normalizedMethod !== "*" && !METHOD_SET.has(normalizedMethod)) return [];
  return [{ type: "http", method: normalizedMethod, path }];
}

function pathFromTarget(target: string): string | null {
  let value = target.trim().replace(/^\{\{[^}]+\}\}/, "");
  if (/^https?:\/\//i.test(value)) {
    value = value.replace(/^https?:\/\/[^/?#]*/i, "");
  }
  value = value.split(/[?#]/)[0] ?? "";
  try {
    value = decodeURI(value);
  } catch {
    // keep as is
  }
  // Postman ":id", Flask/Django "<int:id>", Postman variables "{{id}}" → chi "{id}".
  value = value
    .replace(/\{\{\s*([\w-]+)\s*\}\}/g, "{$1}")
    .replace(/<(?:[\w]+:)?([\w-]+)>/g, "{$1}")
    .replace(/(^|\/):([\w-]+)/g, "$1{$2}");
  if (/\s/.test(value)) return null;
  return "/" + value.replace(/^\/+|\/+$/g, "");
}

function grpcFromPath(target: string): ParsedGrpcEndpoint | null {
  const match = target.trim().match(/^\/?((?:[A-Za-z_]\w*\.)+[A-Za-z_]\w*)\/([A-Za-z_]\w*)$/);
  const [, service, method] = match ?? [];
  if (!service || !method) return null;
  return { type: "grpc", service, method, path: `/${service}/${method}` };
}

// ---- Helpers --------------------------------------------------------------

function result(formats: ImportFormat[], endpoints: ParsedEndpoint[]): ParseResult {
  return { formats, endpoints, skipped: [] };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

// Key used to detect duplicates: path parameters compare equal regardless of
// their names, like the swagger diff on the backend does.
export function endpointMatchKey(endpoint: ParsedEndpoint): string {
  if (endpoint.type === "grpc") return `grpc ${endpoint.path}`;
  const pattern = endpoint.path
    .replace(/^\/+|\/+$/g, "")
    .split("/")
    .map((segment) => (/^(\{.+\}|:.+)$/.test(segment) ? "{}" : segment))
    .join("/");
  return `http ${endpoint.method} /${pattern}`;
}
