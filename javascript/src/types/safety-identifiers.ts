export interface SafetyIdentifierPartition {
  object: "safety_identifier";
  workspace_id: string;
  safety_identifier: string;
  owner_user_id: string;
  status: string;
  created_at: number;
  updated_at: number;
}

export interface ListSafetyIdentifierPartitionsParams {
  owner_user_id?: string;
  status?: string;
}

export interface ListSafetyIdentifierPartitionsResponse {
  object: "list";
  data: SafetyIdentifierPartition[];
}
