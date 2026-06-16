import { buildQuery } from "../internal/query.js";
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
      `/v1/volumes${buildQuery({ limit: params.limit, page_token: params.page_token })}`,
      undefined,
      options,
    );
  }

  create(params: CreateVolumeParams = {}, options?: RequestOptions): Promise<VolumeInfo> {
    return this.http.request<VolumeInfo>("POST", "/v1/volumes", params, options);
  }

  retrieve(volumeID: string, options?: RequestOptions): Promise<VolumeInfo> {
    return this.http.request<VolumeInfo>(
      "GET",
      `/v1/volumes/${encodeURIComponent(volumeID)}`,
      undefined,
      options,
    );
  }

  update(volumeID: string, params: { name: string }, options?: RequestOptions): Promise<VolumeInfo> {
    return this.http.request<VolumeInfo>(
      "PATCH",
      `/v1/volumes/${encodeURIComponent(volumeID)}`,
      params,
      options,
    );
  }

  delete(volumeID: string, options?: RequestOptions): Promise<void> {
    return this.http.requestVoid("DELETE", `/v1/volumes/${encodeURIComponent(volumeID)}`, options);
  }

  listEntries(
    volumeID: string,
    params: ListVolumeEntriesParams = {},
    options?: RequestOptions,
  ): Promise<ListVolumeEntriesResponse> {
    return this.http.request<ListVolumeEntriesResponse>(
      "GET",
      `/v1/volumes/${encodeURIComponent(volumeID)}/entries${buildQuery({
        path: params.path,
        limit: params.limit,
        page_token: params.page_token,
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
      `/v1/volumes/${encodeURIComponent(volumeID)}/search${buildQuery({
        query: params.query,
        path: params.path,
        limit: params.limit,
        page_token: params.page_token,
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
    const url = `/v1/volumes/${encodeURIComponent(volumeID)}/files/${volumePath(path)}${buildQuery({
      max_bytes: params.max_bytes,
      format,
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

  writeFile(volumeID: string, path: string, content: string | ArrayBuffer | Blob, options?: RequestOptions): Promise<VolumeFileWrite> {
    return this.http.requestRaw<VolumeFileWrite>(
      "PUT",
      `/v1/volumes/${encodeURIComponent(volumeID)}/files/${volumePath(path)}`,
      content,
      options,
    );
  }

  deletePath(volumeID: string, path: string, options?: RequestOptions): Promise<VolumePathDelete> {
    return this.http.request<VolumePathDelete>(
      "DELETE",
      `/v1/volumes/${encodeURIComponent(volumeID)}/paths/${volumePath(path)}`,
      undefined,
      options,
    );
  }

  reconcileUsage(volumeID: string, options?: RequestOptions): Promise<VolumeInfo> {
    return this.http.request<VolumeInfo>(
      "POST",
      `/v1/volumes/${encodeURIComponent(volumeID)}/usage/reconcile`,
      undefined,
      options,
    );
  }

  createDirectory(volumeID: string, path: string, options?: RequestOptions): Promise<{ path: string }> {
    return this.http.request<{ path: string }>(
      "POST",
      `/v1/volumes/${encodeURIComponent(volumeID)}/directories`,
      { path },
      options,
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
        `/v1/volumes/${encodeURIComponent(volumeID)}/archive${buildQuery({ path: archivePath || undefined })}`,
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
      `/v1/volumes/${encodeURIComponent(volumeID)}/summarize`,
      params,
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
      `/v1/volumes/${encodeURIComponent(volumeID)}/file_lines/${volumePath(path)}${buildQuery({
        start_line: params.start_line,
        end_line: params.end_line,
        max_bytes: params.max_bytes,
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
      `/v1/volumes/${encodeURIComponent(volumeID)}/file_lines/${volumePath(path)}`,
      params,
      options,
    );
  }

  grep(volumeID: string, params: GrepVolumeParams, options?: RequestOptions): Promise<VolumeGrepResponse> {
    return this.http.request<VolumeGrepResponse>(
      "GET",
      `/v1/volumes/${encodeURIComponent(volumeID)}/grep${buildQuery({
        pattern: params.pattern,
        path: params.path,
        limit: params.limit,
        page_token: params.page_token,
      })}`,
      undefined,
      options,
    );
  }
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
