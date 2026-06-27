export interface SafetyIdentifier {
  object: "safety_identifier";
  workspace_id: string;
  safety_identifier: string;
  created_by_user_id: string;
  status: string;
  created_at: number;
  updated_at: number;
}

export interface ListSafetyIdentifiersParams {
  page_size?: number;
  page_token?: string;
}

export interface ListSafetyIdentifiersResponse {
  object: "list";
  data: SafetyIdentifier[];
}
