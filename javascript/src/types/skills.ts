export type SkillSourceType =
  | "SKILL_SOURCE_TYPE_PLATFORM"
  | "SKILL_SOURCE_TYPE_LOCAL"
  | "SKILL_SOURCE_TYPE_GENERATED"
  | string;

export type SkillBranch = "main" | "dev" | string;
export type SkillAcceptStrategy = "patch" | "mirror";
export type SkillBranchDiffStatus = "added" | "modified" | "deleted" | "unchanged" | string;

export interface SkillReference {
  skill_id: string;
  branch?: SkillBranch;
}

export interface LocalSkillDescriptor {
  local_skill_id: string;
  skill_ref?: string;
  name?: string;
  description?: string;
  root_hint?: string;
  digest?: string;
  manifest?: string;
  manifest_truncated?: boolean;
  metadata?: Record<string, unknown>;
}

export interface Skill {
  object: "skill";
  skill_id: string;
  tenant_id?: string;
  name: string;
  description?: string;
  source_type?: SkillSourceType;
  main_digest?: string;
  dev_digest?: string;
  has_dev?: boolean;
  dev_updated_at?: number;
  dev_source_response_id?: string;
  created_by_user_id?: string;
  updated_by_user_id?: string;
  created_at?: number;
  updated_at?: number;
  metadata?: Record<string, unknown>;
  archived?: boolean;
}

export interface SkillSummary {
  object: "skill_summary";
  skill_id: string;
  skill_ref?: string;
  name?: string;
  description?: string;
  source_type?: SkillSourceType;
  branch?: SkillBranch;
  digest?: string;
  artifact_uri?: string;
  has_dev?: boolean;
  metadata?: Record<string, unknown>;
}

export interface FocusedSkill extends Omit<SkillSummary, "object"> {
  object: "focused_skill";
  mount_hint?: string;
  manifest?: string;
  manifest_truncated?: boolean;
  entries?: SkillFileEntry[];
  files?: SkillFocusedFile[];
}

export interface SkillFocusedFile {
  path: string;
  content?: string;
  truncated?: boolean;
  size?: number;
  branch?: SkillBranch;
  error?: SkillOperationError;
}

export interface SkillFocusItem {
  skill_id?: string;
  branch?: SkillBranch;
  paths?: string[];
  include_manifest?: boolean;
  local_skill?: LocalSkillDescriptor;
}

export interface SkillOperationError {
  code?: string;
  message?: string;
}

export interface SkillFocusResultItem {
  ok: boolean;
  skill_ref?: string;
  skill_id?: string;
  local_skill_id?: string;
  branch?: SkillBranch;
  skill?: FocusedSkill;
  error?: SkillOperationError;
}

export interface SkillFocusResponse {
  object: "skill_focus_result";
  data: SkillFocusResultItem[];
}

export interface SkillFileEntry {
  path: string;
  is_dir: boolean;
  size: number;
  modified_at?: number;
}

export interface ListSkillsParams {
  include_archived?: boolean;
  limit?: number;
  page_token?: string;
}

export interface ListSkillsResponse {
  object: "list";
  data: Skill[];
  next_page_token?: string;
}

export interface ListSkillSummariesResponse {
  object: "list";
  data: SkillSummary[];
  next_page_token?: string;
}

export interface DiscoverSkillsParams {
  query?: string;
  branch?: SkillBranch | "both";
  include_dev?: boolean;
  limit?: number;
  local_skills?: LocalSkillDescriptor[];
}

export interface FocusSkillParams {
  skills: SkillFocusItem[];
  fallback_to_main?: boolean;
  max_manifest_chars?: number;
  max_file_chars?: number;
}

export interface SkillFileMutation {
  path: string;
  content?: string;
  content_base64?: string;
}

export interface CreateSkillDevParams {
  name: string;
  description?: string;
  metadata?: Record<string, unknown>;
  files?: SkillFileMutation[];
}

export interface CreateSkillDevResponse {
  object: "skill_create_result";
  skill: Skill;
  branch: SkillBranch;
  files: Array<{ path: string; size: number }>;
  focused_skill?: FocusedSkill;
}

export interface SkillFileUpdateMutation extends SkillFileMutation {
  skill_id: string;
}

export interface UpdateSkillFilePrimitiveParams {
  updates: SkillFileUpdateMutation[];
}

export interface SkillUpdateResultItem {
  ok: boolean;
  skill_id?: string;
  path?: string;
  size?: number;
  skill?: SkillSummary;
  error?: SkillOperationError;
}

export interface UpdateSkillFilePrimitiveResponse {
  object: "skill_update_result";
  data: SkillUpdateResultItem[];
}

export interface ListSkillFilesParams {
  path?: string;
  branch?: SkillBranch;
  fallback_to_main?: boolean;
  limit?: number;
  page_token?: string;
}

export interface ListSkillFilesResponse {
  object: "list";
  entries: SkillFileEntry[];
  next_page_token?: string;
}

export interface SkillFile {
  object: "skill_file";
  path?: string;
  branch?: SkillBranch;
  content?: string;
  size: number;
  truncated?: boolean;
  recursive?: boolean;
}

export interface SkillArchiveParams {
  path?: string;
  branch?: SkillBranch;
  fallback_to_main?: boolean;
}

export interface SkillArchive {
  path: string;
  content: ArrayBuffer;
  content_type?: string;
}

export interface ImportSkillArchiveParams {
  path?: string;
  branch?: SkillBranch;
  replace?: boolean;
  strip_top_level_dir?: boolean;
}

export interface SkillImportResponse {
  object: "skill_import_result";
  branch: SkillBranch;
  file_count: number;
  byte_count: number;
  skill: Skill;
}

export interface SkillBranchDiffParams {
  path?: string;
  max_file_chars?: number;
  include_unchanged?: boolean;
}

export interface SkillBranchDiffFile {
  path: string;
  status: SkillBranchDiffStatus;
  base_size?: number;
  compare_size?: number;
  text?: boolean;
  binary?: boolean;
  too_large?: boolean;
  truncated?: boolean;
  diff?: string;
}

export interface SkillBranchDiff {
  object: "skill_branch_diff";
  skill_id: string;
  base_branch: SkillBranch;
  compare_branch: SkillBranch;
  path?: string;
  summary: {
    added: number;
    modified: number;
    deleted: number;
    unchanged: number;
  };
  files: SkillBranchDiffFile[];
}

export interface PushSkillDirectoryParams extends ImportSkillArchiveParams {}

export interface PullSkillDirectoryParams extends SkillArchiveParams {
  replace?: boolean;
}

export interface SkillDirectoryPullResult {
  path: string;
  file_count: number;
  byte_count: number;
}

export interface CreateSkillParams {
  name?: string;
  description?: string;
  metadata?: Record<string, unknown>;
}

export interface UpdateSkillParams {
  name?: string;
  description?: string;
  metadata?: Record<string, unknown>;
}
