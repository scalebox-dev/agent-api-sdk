export interface SafetyIdentifierPartition {
  object: "safety_identifier";
  workspace_id: string;
  safety_identifier: string;
  created_by_user_id: string;
  status: string;
  created_at: number;
  updated_at: number;
}

export interface ListSafetyIdentifierPartitionsParams {
  page_size?: number;
  page_token?: string;
}

export interface ListSafetyIdentifierPartitionsResponse {
  object: "list";
  data: SafetyIdentifierPartition[];
}
