export interface VolumeInfo {
  volume_id: string;
  tenant_id?: string;
  safety_identifier?: string;
  name?: string;
  oss_prefix?: string;
  bytes_used?: number;
  object_count?: number;
  usage_reconciled_at_unix?: number;
  created_at_unix?: number;
  updated_at_unix?: number;
}

export interface VolumeEntry {
  path: string;
  is_dir: boolean;
  size: number;
  modified_at_unix?: number;
}

export interface ListVolumesParams {
  limit?: number;
  page_token?: string;
  safety_identifier?: string;
}

export interface ListVolumesResponse {
  object: "list";
  data: VolumeInfo[];
  next_page_token?: string;
}

export interface CreateVolumeParams {
  name?: string;
  safety_identifier?: string;
}

export interface UpdateVolumeParams {
  name?: string;
  safety_identifier?: string;
  new_safety_identifier?: string;
}

export interface ListVolumeEntriesParams {
  path?: string;
  limit?: number;
  page_token?: string;
  safety_identifier?: string;
}

export interface SearchVolumeEntriesParams extends ListVolumeEntriesParams {
  query: string;
}

export interface ListVolumeEntriesResponse {
  object: "list";
  entries: VolumeEntry[];
  next_page_token?: string;
}

export interface ReadVolumeFileParams {
  max_bytes?: number;
  safety_identifier?: string;
}

export interface ReadVolumeFileRawParams {
  max_bytes?: number;
  format: "raw";
  safety_identifier?: string;
}

export type VolumeFileEncoding = "text" | "extracted_text" | "url" | "base64";

export interface VolumeFileDeliver {
  path: string;
  encoding: VolumeFileEncoding;
  mime_type: string;
  size: number;
  truncated: boolean;
  content?: string;
  content_base64?: string;
  image_url?: string;
  expires_at_unix?: number;
  extraction_warnings?: string[];
}

export interface VolumeFileRaw {
  path: string;
  size: number;
  truncated: boolean;
  content: ArrayBuffer;
  content_type?: string;
}

export interface VolumeFileWrite {
  path: string;
  size: number;
}

export interface VolumePathDelete {
  path: string;
  recursive: boolean;
}

export interface DownloadVolumeArchiveParams {
  path?: string;
  safety_identifier?: string;
}

export interface VolumeArchive {
  path: string;
  content: ArrayBuffer;
  content_type?: string;
}

export interface SummarizeVolumeParams {
  path?: string;
  safety_identifier?: string;
}

export interface VolumeSummaryPreview {
  path: string;
  size: number;
  preview: string;
  preview_truncated?: boolean;
}

export interface VolumeSummary {
  summary_path: string;
  file_count: number;
  total_bytes: number;
  top_paths_by_size: string[];
  text_previews: VolumeSummaryPreview[];
  generated_at_unix: number;
}

export interface ReadVolumeFileLinesParams {
  start_line: number;
  end_line?: number;
  max_bytes?: number;
  safety_identifier?: string;
}

export interface VolumeFileLines {
  path: string;
  start_line: number;
  end_line: number;
  total_lines: number;
  lines: string[];
  file_truncated: boolean;
  size: number;
}

export interface PatchVolumeFileLinesParams {
  start_line: number;
  end_line?: number;
  replacement?: string;
  safety_identifier?: string;
}

export interface VolumeFileLinesPatch {
  path: string;
  start_line: number;
  end_line: number;
  total_lines: number;
  size: number;
}

export interface GrepVolumeParams {
  pattern: string;
  path?: string;
  limit?: number;
  page_token?: string;
  safety_identifier?: string;
}

export interface VolumeGrepMatch {
  path: string;
  line_number: number;
  line: string;
}

export interface VolumeGrepResponse {
  object: "list";
  matches: VolumeGrepMatch[];
  next_page_token?: string;
  files_scanned: number;
  scan_truncated: boolean;
}
