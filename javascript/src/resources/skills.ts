import { buildQuery } from "../internal/query.js";
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
      `/v1/skills${buildQuery({
        include_archived: params.include_archived ? "true" : undefined,
        limit: params.limit,
        page_token: params.page_token,
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

  retrieve(skillID: string, options?: RequestOptions): Promise<Skill> {
    return this.http.request<Skill>("GET", `/v1/skills/${encodeURIComponent(skillID)}`, undefined, options);
  }

  update(skillID: string, params: UpdateSkillParams, options?: RequestOptions): Promise<Skill> {
    return this.http.request<Skill>("PATCH", `/v1/skills/${encodeURIComponent(skillID)}`, params, options);
  }

  archive(skillID: string, options?: RequestOptions): Promise<Skill> {
    return this.http.request<Skill>("POST", `/v1/skills/${encodeURIComponent(skillID)}/archive`, {}, options);
  }

  delete(skillID: string, options?: RequestOptions): Promise<{ deleted: boolean }> {
    return this.http.request<{ deleted: boolean }>("DELETE", `/v1/skills/${encodeURIComponent(skillID)}`, undefined, options);
  }

  acceptDev(skillID: string, params: { strategy?: SkillAcceptStrategy } = {}, options?: RequestOptions): Promise<Skill> {
    return this.http.request<Skill>(
      "POST",
      `/v1/skills/${encodeURIComponent(skillID)}/accept_dev${buildQuery({ strategy: params.strategy })}`,
      {},
      options,
    );
  }

  discardDev(skillID: string, options?: RequestOptions): Promise<Skill> {
    return this.http.request<Skill>("POST", `/v1/skills/${encodeURIComponent(skillID)}/discard_dev`, {}, options);
  }

  listFiles(skillID: string, params: ListSkillFilesParams = {}, options?: RequestOptions): Promise<ListSkillFilesResponse> {
    return this.http.request<ListSkillFilesResponse>(
      "GET",
      `/v1/skills/${encodeURIComponent(skillID)}/files${buildQuery({
        path: params.path,
        branch: params.branch,
        fallback_to_main: params.fallback_to_main === false ? "false" : undefined,
        limit: params.limit,
        page_token: params.page_token,
      })}`,
      undefined,
      options,
    );
  }

  readFile(skillID: string, path: string, params: { branch?: SkillBranch; fallback_to_main?: boolean; max_bytes?: number } = {}, options?: RequestOptions): Promise<SkillFile> {
    return this.http.request<SkillFile>(
      "GET",
      `/v1/skills/${encodeURIComponent(skillID)}/files/${skillPath(path)}${buildQuery({
        branch: params.branch,
        fallback_to_main: params.fallback_to_main === false ? "false" : undefined,
        max_bytes: params.max_bytes,
      })}`,
      undefined,
      options,
    );
  }

  writeFile(skillID: string, path: string, content: string | ArrayBuffer | Blob, params: { branch?: SkillBranch } = {}, options?: RequestOptions): Promise<SkillFile> {
    return this.http.requestRaw<SkillFile>(
      "PUT",
      `/v1/skills/${encodeURIComponent(skillID)}/files/${skillPath(path)}${buildQuery({ branch: params.branch })}`,
      content,
      options,
    );
  }

  deleteFile(skillID: string, path: string, params: { branch?: SkillBranch } = {}, options?: RequestOptions): Promise<SkillFile> {
    return this.http.request<SkillFile>(
      "DELETE",
      `/v1/skills/${encodeURIComponent(skillID)}/files/${skillPath(path)}${buildQuery({ branch: params.branch })}`,
      undefined,
      options,
    );
  }

  exportArchive(skillID: string, params: SkillArchiveParams = {}, options?: RequestOptions): Promise<SkillArchive> {
    const archivePath = normalizeArchivePath(params.path);
    return this.http
      .requestBinary(
        "GET",
        `/v1/skills/${encodeURIComponent(skillID)}/export${buildQuery({
          path: archivePath || undefined,
          branch: params.branch,
          fallback_to_main: params.fallback_to_main === false ? "false" : undefined,
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
      `/v1/skills/${encodeURIComponent(skillID)}/import${buildQuery({
        path: normalizeArchivePath(params.path) || undefined,
        branch: params.branch,
        replace: params.replace ? "true" : undefined,
        strip_top_level_dir: params.strip_top_level_dir === false ? "false" : undefined,
      })}`,
      archive,
      options,
    );
  }

  diff(skillID: string, params: SkillBranchDiffParams = {}, options?: RequestOptions): Promise<SkillBranchDiff> {
    return this.http.request<SkillBranchDiff>(
      "GET",
      `/v1/skills/${encodeURIComponent(skillID)}/diff${buildQuery({
        path: normalizeArchivePath(params.path) || undefined,
        max_file_chars: params.max_file_chars,
        include_unchanged: params.include_unchanged ? "true" : undefined,
      })}`,
      undefined,
      options,
    );
  }

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
