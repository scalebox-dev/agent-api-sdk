import type { HTTPClient } from "../internal/http.js";
import type { RequestOptions } from "../types/common.js";
import type {
  CreateVolumeParams,
  DownloadVolumeArchiveParams,
  GrepVolumeParams,
  ListVolumeEntriesParams,
  ListVolumeEntriesResponse,
  ListVolumesParams,
  ListVolumesResponse,
  PatchVolumeFileLinesParams,
  ReadVolumeFileLinesParams,
  ReadVolumeFileParams,
  ReadVolumeFileRawParams,
  SearchVolumeEntriesParams,
  SummarizeVolumeParams,
  UpdateVolumeParams,
  VolumeArchive,
  VolumeFileDeliver,
  VolumeFileLines,
  VolumeFileLinesPatch,
  VolumeFileRaw,
  VolumeFileWrite,
  VolumeGrepResponse,
  VolumeInfo,
  VolumePathDelete,
  VolumeSummary,
} from "../types/volumes.js";

export class VolumesResource {
  constructor(private readonly http: HTTPClient) {}

  list(params: ListVolumesParams = {}, options?: RequestOptions): Promise<ListVolumesResponse> {
    return this.http.request<ListVolumesResponse>(
      "GET",
      `/v1/volumes${buildSafetyQuery({ limit: params.limit, page_token: params.page_token, safety_identifier: params.safety_identifier })}`,
      undefined,
      options,
    );
  }

  create(params: CreateVolumeParams = {}, options?: RequestOptions): Promise<VolumeInfo> {
    return this.http.request<VolumeInfo>("POST", "/v1/volumes", params, options);
  }

  retrieve(volumeID: string, paramsOrOptions?: SafetyParams | RequestOptions, options?: RequestOptions): Promise<VolumeInfo> {
    const [params, requestOptions] = splitSafetyParams(paramsOrOptions, options);
    return this.http.request<VolumeInfo>(
      "GET",
      `/v1/volumes/${encodeURIComponent(volumeID)}${buildSafetyQuery({ safety_identifier: params.safety_identifier })}`,
      undefined,
      requestOptions,
    );
  }

  update(volumeID: string, params: UpdateVolumeParams, options?: RequestOptions): Promise<VolumeInfo> {
    const { safety_identifier, new_safety_identifier, ...body } = params;
    const requestBody: { name?: string; safety_identifier?: string } = body;
    if (new_safety_identifier !== undefined) {
      requestBody.safety_identifier = new_safety_identifier;
    }
    return this.http.request<VolumeInfo>(
      "PATCH",
      `/v1/volumes/${encodeURIComponent(volumeID)}${buildSafetyQuery({ safety_identifier }, new_safety_identifier !== undefined)}`,
      requestBody,
      options,
    );
  }

  delete(volumeID: string, paramsOrOptions?: SafetyParams | RequestOptions, options?: RequestOptions): Promise<void> {
    const [params, requestOptions] = splitSafetyParams(paramsOrOptions, options);
    return this.http.requestVoid(
      "DELETE",
      `/v1/volumes/${encodeURIComponent(volumeID)}${buildSafetyQuery({ safety_identifier: params.safety_identifier })}`,
      requestOptions,
    );
  }

  listEntries(
    volumeID: string,
    params: ListVolumeEntriesParams = {},
    options?: RequestOptions,
  ): Promise<ListVolumeEntriesResponse> {
    return this.http.request<ListVolumeEntriesResponse>(
      "GET",
      `/v1/volumes/${encodeURIComponent(volumeID)}/entries${buildSafetyQuery({
        path: params.path,
        limit: params.limit,
        page_token: params.page_token,
        safety_identifier: params.safety_identifier,
      })}`,
      undefined,
      options,
    );
  }

  searchEntries(
    volumeID: string,
    params: SearchVolumeEntriesParams,
    options?: RequestOptions,
  ): Promise<ListVolumeEntriesResponse> {
    return this.http.request<ListVolumeEntriesResponse>(
      "GET",
      `/v1/volumes/${encodeURIComponent(volumeID)}/search${buildSafetyQuery({
        query: params.query,
        path: params.path,
        limit: params.limit,
        page_token: params.page_token,
        safety_identifier: params.safety_identifier,
      })}`,
      undefined,
      options,
    );
  }

  readFile(
    volumeID: string,
    path: string,
    params: ReadVolumeFileRawParams,
    options?: RequestOptions,
  ): Promise<VolumeFileRaw>;
  readFile(
    volumeID: string,
    path: string,
    params?: ReadVolumeFileParams,
    options?: RequestOptions,
  ): Promise<VolumeFileDeliver>;
  readFile(
    volumeID: string,
    path: string,
    params: ReadVolumeFileParams | ReadVolumeFileRawParams = {},
    options?: RequestOptions,
  ): Promise<VolumeFileDeliver | VolumeFileRaw> {
    const format = "format" in params ? params.format : undefined;
    const url = `/v1/volumes/${encodeURIComponent(volumeID)}/files/${volumePath(path)}${buildSafetyQuery({
      max_bytes: params.max_bytes,
      format,
      safety_identifier: params.safety_identifier,
    })}`;
    if (format === "raw") {
      return this.http.requestBinary("GET", url, options).then(({ body, headers }) => ({
        path,
        size: Number(headers.get("X-Volume-Size") ?? body.byteLength),
        truncated: headers.get("X-Volume-Truncated") === "true",
        content: body,
        content_type: headers.get("Content-Type") ?? undefined,
      }));
    }
    return this.http.request<VolumeFileDeliver>("GET", url, undefined, options);
  }

  writeFile(
    volumeID: string,
    path: string,
    content: string | ArrayBuffer | Blob,
    paramsOrOptions?: SafetyParams | RequestOptions,
    options?: RequestOptions,
  ): Promise<VolumeFileWrite> {
    const [params, requestOptions] = splitSafetyParams(paramsOrOptions, options);
    return this.http.requestRaw<VolumeFileWrite>(
      "PUT",
      `/v1/volumes/${encodeURIComponent(volumeID)}/files/${volumePath(path)}${buildSafetyQuery({ safety_identifier: params.safety_identifier })}`,
      content,
      requestOptions,
    );
  }

  deletePath(volumeID: string, path: string, paramsOrOptions?: SafetyParams | RequestOptions, options?: RequestOptions): Promise<VolumePathDelete> {
    const [params, requestOptions] = splitSafetyParams(paramsOrOptions, options);
    return this.http.request<VolumePathDelete>(
      "DELETE",
      `/v1/volumes/${encodeURIComponent(volumeID)}/paths/${volumePath(path)}${buildSafetyQuery({ safety_identifier: params.safety_identifier })}`,
      undefined,
      requestOptions,
    );
  }

  reconcileUsage(volumeID: string, paramsOrOptions?: SafetyParams | RequestOptions, options?: RequestOptions): Promise<VolumeInfo> {
    const [params, requestOptions] = splitSafetyParams(paramsOrOptions, options);
    return this.http.request<VolumeInfo>(
      "POST",
      `/v1/volumes/${encodeURIComponent(volumeID)}/usage/reconcile${buildSafetyQuery({ safety_identifier: params.safety_identifier })}`,
      undefined,
      requestOptions,
    );
  }

  createDirectory(volumeID: string, path: string, paramsOrOptions?: SafetyParams | RequestOptions, options?: RequestOptions): Promise<{ path: string }> {
    const [params, requestOptions] = splitSafetyParams(paramsOrOptions, options);
    return this.http.request<{ path: string }>(
      "POST",
      `/v1/volumes/${encodeURIComponent(volumeID)}/directories${buildSafetyQuery({ safety_identifier: params.safety_identifier })}`,
      { path },
      requestOptions,
    );
  }

  downloadArchive(
    volumeID: string,
    params: DownloadVolumeArchiveParams = {},
    options?: RequestOptions,
  ): Promise<VolumeArchive> {
    const archivePath = normalizeArchivePath(params.path);
    return this.http
      .requestBinary(
        "GET",
        `/v1/volumes/${encodeURIComponent(volumeID)}/archive${buildSafetyQuery({ path: archivePath || undefined, safety_identifier: params.safety_identifier })}`,
        options,
      )
      .then(({ body, headers }) => ({
        path: archivePath,
        content: body,
        content_type: headers.get("Content-Type") ?? undefined,
      }));
  }

  summarize(volumeID: string, params: SummarizeVolumeParams = {}, options?: RequestOptions): Promise<VolumeSummary> {
    return this.http.request<VolumeSummary>(
      "POST",
      `/v1/volumes/${encodeURIComponent(volumeID)}/summarize${buildSafetyQuery({ safety_identifier: params.safety_identifier })}`,
      { path: params.path },
      options,
    );
  }

  readLines(
    volumeID: string,
    path: string,
    params: ReadVolumeFileLinesParams,
    options?: RequestOptions,
  ): Promise<VolumeFileLines> {
    return this.http.request<VolumeFileLines>(
      "GET",
      `/v1/volumes/${encodeURIComponent(volumeID)}/file_lines/${volumePath(path)}${buildSafetyQuery({
        start_line: params.start_line,
        end_line: params.end_line,
        max_bytes: params.max_bytes,
        safety_identifier: params.safety_identifier,
      })}`,
      undefined,
      options,
    );
  }

  patchLines(
    volumeID: string,
    path: string,
    params: PatchVolumeFileLinesParams,
    options?: RequestOptions,
  ): Promise<VolumeFileLinesPatch> {
    return this.http.request<VolumeFileLinesPatch>(
      "PATCH",
      `/v1/volumes/${encodeURIComponent(volumeID)}/file_lines/${volumePath(path)}${buildSafetyQuery({ safety_identifier: params.safety_identifier })}`,
      { start_line: params.start_line, end_line: params.end_line, replacement: params.replacement },
      options,
    );
  }

  grep(volumeID: string, params: GrepVolumeParams, options?: RequestOptions): Promise<VolumeGrepResponse> {
    return this.http.request<VolumeGrepResponse>(
      "GET",
      `/v1/volumes/${encodeURIComponent(volumeID)}/grep${buildSafetyQuery({
        pattern: params.pattern,
        path: params.path,
        limit: params.limit,
        page_token: params.page_token,
        safety_identifier: params.safety_identifier,
      })}`,
      undefined,
      options,
    );
  }
}

type SafetyParams = { safety_identifier?: string };

function splitSafetyParams(paramsOrOptions?: SafetyParams | RequestOptions, options?: RequestOptions): [SafetyParams, RequestOptions | undefined] {
  if (!paramsOrOptions) {
    return [{}, options];
  }
  if (
    "headers" in paramsOrOptions ||
    "timeout" in paramsOrOptions ||
    "signal" in paramsOrOptions ||
    "maxRetries" in paramsOrOptions
  ) {
    return [{}, paramsOrOptions];
  }
  return [paramsOrOptions as SafetyParams, options];
}

function buildSafetyQuery(values: Record<string, string | number | undefined>, forceSafety = false): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined && (value !== "" || (forceSafety && key === "safety_identifier"))) {
      search.set(key, String(value));
    }
  }
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

function volumePath(path: string): string {
  return path
    .split("/")
    .filter(Boolean)
    .map((part) => encodeURIComponent(part))
    .join("/");
}

function normalizeArchivePath(path?: string): string {
  if (!path) {
    return "";
  }
  return path.trim().replace(/^\/+/, "").replace(/\/+/g, "/").replace(/\/$/, "");
}
