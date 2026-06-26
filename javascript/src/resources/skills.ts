import type { HTTPClient } from "../internal/http.js";
import type { RequestOptions } from "../types/common.js";
import type {
  CreateSkillParams,
  CreateSkillDevParams,
  CreateSkillDevResponse,
  DiscoverSkillsParams,
  FocusSkillParams,
  ImportSkillArchiveParams,
  ListSkillFilesParams,
  ListSkillFilesResponse,
  ListSkillsParams,
  ListSkillsResponse,
  ListSkillSummariesResponse,
  Skill,
  SkillAcceptStrategy,
  SkillArchive,
  SkillArchiveParams,
  SkillBranchDiff,
  SkillBranchDiffParams,
  SkillFile,
  SkillBranch,
  SkillFocusResponse,
  SkillImportResponse,
  UpdateSkillFilePrimitiveParams,
  UpdateSkillFilePrimitiveResponse,
  UpdateSkillParams,
} from "../types/skills.js";

export class SkillsResource {
  constructor(private readonly http: HTTPClient) {}

  list(params: ListSkillsParams = {}, options?: RequestOptions): Promise<ListSkillsResponse> {
    return this.http.request<ListSkillsResponse>(
      "GET",
      `/v1/skills${buildSkillQuery({
        include_archived: params.include_archived ? "true" : undefined,
        limit: params.limit,
        page_token: params.page_token,
        safety_identifier: params.safety_identifier,
      })}`,
      undefined,
      options,
    );
  }

  create(params: CreateSkillParams = {}, options?: RequestOptions): Promise<Skill> {
    return this.http.request<Skill>("POST", "/v1/skills", params, options);
  }

  discover(params: DiscoverSkillsParams = {}, options?: RequestOptions): Promise<ListSkillSummariesResponse> {
    return this.http.request<ListSkillSummariesResponse>("POST", "/v1/skills/discover", params, options);
  }

  focus(params: FocusSkillParams, options?: RequestOptions): Promise<SkillFocusResponse> {
    return this.http.request<SkillFocusResponse>("POST", "/v1/skills/focus", params, options);
  }

  createDev(params: CreateSkillDevParams, options?: RequestOptions): Promise<CreateSkillDevResponse> {
    return this.http.request<CreateSkillDevResponse>("POST", "/v1/skills/create_dev", params, options);
  }

  updateFile(params: UpdateSkillFilePrimitiveParams, options?: RequestOptions): Promise<UpdateSkillFilePrimitiveResponse> {
    return this.http.request<UpdateSkillFilePrimitiveResponse>("POST", "/v1/skills/update_file", params, options);
  }

  retrieve(skillID: string, paramsOrOptions?: SafetyParams | RequestOptions, options?: RequestOptions): Promise<Skill> {
    const [params, requestOptions] = splitSafetyParams(paramsOrOptions, options);
    return this.http.request<Skill>(
      "GET",
      `/v1/skills/${encodeURIComponent(skillID)}${buildSkillQuery({ safety_identifier: params.safety_identifier })}`,
      undefined,
      requestOptions,
    );
  }

  update(skillID: string, params: UpdateSkillParams, options?: RequestOptions): Promise<Skill> {
    const { safety_identifier, new_safety_identifier, ...body } = params;
    const requestBody: { name?: string; description?: string; metadata?: Record<string, unknown>; safety_identifier?: string } = body;
    if (new_safety_identifier !== undefined) {
      requestBody.safety_identifier = new_safety_identifier;
    }
    return this.http.request<Skill>(
      "PATCH",
      `/v1/skills/${encodeURIComponent(skillID)}${buildSkillQuery({ safety_identifier }, new_safety_identifier !== undefined)}`,
      requestBody,
      options,
    );
  }

  archive(skillID: string, paramsOrOptions?: SafetyParams | RequestOptions, options?: RequestOptions): Promise<Skill> {
    const [params, requestOptions] = splitSafetyParams(paramsOrOptions, options);
    return this.http.request<Skill>(
      "POST",
      `/v1/skills/${encodeURIComponent(skillID)}/archive${buildSkillQuery({ safety_identifier: params.safety_identifier })}`,
      {},
      requestOptions,
    );
  }

  delete(skillID: string, paramsOrOptions?: SafetyParams | RequestOptions, options?: RequestOptions): Promise<{ deleted: boolean }> {
    const [params, requestOptions] = splitSafetyParams(paramsOrOptions, options);
    return this.http.request<{ deleted: boolean }>(
      "DELETE",
      `/v1/skills/${encodeURIComponent(skillID)}${buildSkillQuery({ safety_identifier: params.safety_identifier })}`,
      undefined,
      requestOptions,
    );
  }

  acceptDev(skillID: string, params: { strategy?: SkillAcceptStrategy; safety_identifier?: string } = {}, options?: RequestOptions): Promise<Skill> {
    return this.http.request<Skill>(
      "POST",
      `/v1/skills/${encodeURIComponent(skillID)}/accept_dev${buildSkillQuery({ strategy: params.strategy, safety_identifier: params.safety_identifier })}`,
      {},
      options,
    );
  }

  discardDev(skillID: string, paramsOrOptions?: SafetyParams | RequestOptions, options?: RequestOptions): Promise<Skill> {
    const [params, requestOptions] = splitSafetyParams(paramsOrOptions, options);
    return this.http.request<Skill>(
      "POST",
      `/v1/skills/${encodeURIComponent(skillID)}/discard_dev${buildSkillQuery({ safety_identifier: params.safety_identifier })}`,
      {},
      requestOptions,
    );
  }

  listFiles(skillID: string, params: ListSkillFilesParams = {}, options?: RequestOptions): Promise<ListSkillFilesResponse> {
    return this.http.request<ListSkillFilesResponse>(
      "GET",
      `/v1/skills/${encodeURIComponent(skillID)}/files${buildSkillQuery({
        path: params.path,
        branch: params.branch,
        fallback_to_main: params.fallback_to_main === false ? "false" : undefined,
        limit: params.limit,
        page_token: params.page_token,
        safety_identifier: params.safety_identifier,
      })}`,
      undefined,
      options,
    );
  }

  readFile(skillID: string, path: string, params: { branch?: SkillBranch; fallback_to_main?: boolean; max_bytes?: number; safety_identifier?: string } = {}, options?: RequestOptions): Promise<SkillFile> {
    return this.http.request<SkillFile>(
      "GET",
      `/v1/skills/${encodeURIComponent(skillID)}/files/${skillPath(path)}${buildSkillQuery({
        branch: params.branch,
        fallback_to_main: params.fallback_to_main === false ? "false" : undefined,
        max_bytes: params.max_bytes,
        safety_identifier: params.safety_identifier,
      })}`,
      undefined,
      options,
    );
  }

  writeFile(skillID: string, path: string, content: string | ArrayBuffer | Blob, params: { branch?: SkillBranch; safety_identifier?: string } = {}, options?: RequestOptions): Promise<SkillFile> {
    return this.http.requestRaw<SkillFile>(
      "PUT",
      `/v1/skills/${encodeURIComponent(skillID)}/files/${skillPath(path)}${buildSkillQuery({ branch: params.branch, safety_identifier: params.safety_identifier })}`,
      content,
      options,
    );
  }

  deleteFile(skillID: string, path: string, params: { branch?: SkillBranch; safety_identifier?: string } = {}, options?: RequestOptions): Promise<SkillFile> {
    return this.http.request<SkillFile>(
      "DELETE",
      `/v1/skills/${encodeURIComponent(skillID)}/files/${skillPath(path)}${buildSkillQuery({ branch: params.branch, safety_identifier: params.safety_identifier })}`,
      undefined,
      options,
    );
  }

  exportArchive(skillID: string, params: SkillArchiveParams = {}, options?: RequestOptions): Promise<SkillArchive> {
    const archivePath = normalizeArchivePath(params.path);
    return this.http
      .requestBinary(
        "GET",
        `/v1/skills/${encodeURIComponent(skillID)}/export${buildSkillQuery({
          path: archivePath || undefined,
          branch: params.branch,
          fallback_to_main: params.fallback_to_main === false ? "false" : undefined,
          safety_identifier: params.safety_identifier,
        })}`,
        options,
      )
      .then(({ body, headers }) => ({
        path: archivePath,
        content: body,
        content_type: headers.get("Content-Type") ?? undefined,
      }));
  }

  importArchive(
    skillID: string,
    archive: ArrayBuffer | Blob,
    params: ImportSkillArchiveParams = {},
    options?: RequestOptions,
  ): Promise<SkillImportResponse> {
    return this.http.requestRaw<SkillImportResponse>(
      "POST",
      `/v1/skills/${encodeURIComponent(skillID)}/import${buildSkillQuery({
        path: normalizeArchivePath(params.path) || undefined,
        branch: params.branch,
        replace: params.replace ? "true" : undefined,
        strip_top_level_dir: params.strip_top_level_dir === false ? "false" : undefined,
        safety_identifier: params.safety_identifier,
      })}`,
      archive,
      options,
    );
  }

  diff(skillID: string, params: SkillBranchDiffParams = {}, options?: RequestOptions): Promise<SkillBranchDiff> {
    return this.http.request<SkillBranchDiff>(
      "GET",
      `/v1/skills/${encodeURIComponent(skillID)}/diff${buildSkillQuery({
        path: normalizeArchivePath(params.path) || undefined,
        max_file_chars: params.max_file_chars,
        include_unchanged: params.include_unchanged ? "true" : undefined,
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

function buildSkillQuery(values: Record<string, string | number | undefined>, forceSafety = false): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined && (value !== "" || (forceSafety && key === "safety_identifier"))) {
      search.set(key, String(value));
    }
  }
  const qs = search.toString();
  return qs ? `?${qs}` : "";
}

function skillPath(path: string): string {
  return path
    .split("/")
    .filter(Boolean)
    .map((part) => encodeURIComponent(part))
    .join("/");
}

function normalizeArchivePath(path?: string): string {
  return (path ?? "").trim().replace(/^\/+|\/+$/g, "");
}
