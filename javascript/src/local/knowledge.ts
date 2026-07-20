import type { FunctionTool } from "../types/tools.js";

export type LocalKnowledgeSourceType = "transcript" | "workdir_file" | "note" | "tool_output";

export interface LocalKnowledgeScope {
  conversationId?: string;
  workspaceId?: string;
  profile?: string;
  workdir?: string;
  tags?: string[];
}

export interface LocalKnowledgeRetentionPolicy {
  /** Maximum age for transcript-derived sources, in seconds. */
  transcriptTtlSeconds?: number;
  /** Maximum age for workdir file sources that have not been refreshed, in seconds. */
  workdirTtlSeconds?: number;
  /** Maximum total stored local knowledge bytes before oldest sources are pruned. */
  maxBytes?: number;
  /** Maximum transcript-derived sources to retain. */
  maxTranscriptSources?: number;
  /** Maximum workdir file sources to retain. */
  maxWorkdirSources?: number;
  /** Remove soft-deleted sources older than this many seconds. */
  deletedTtlSeconds?: number;
}

export interface LocalKnowledgeRetrievalPolicy {
  defaultLimit?: number;
  maxLimit?: number;
  defaultContextBytes?: number;
  maxContextBytes?: number;
  /** prefer ranks active scope first; filter excludes records outside the active scope. */
  scopeMode?: "prefer" | "filter";
  /** In filter mode, allow same workspace/profile knowledge from other conversations. */
  includeConversationSiblings?: boolean;
}

export interface LocalKnowledgeIngestionPolicy {
  maxTranscriptBytes?: number;
  maxWorkdirFiles?: number;
  maxWorkdirFileBytes?: number;
  maxChunkBytes?: number;
  includeWorkdir?: boolean;
  includeTranscripts?: boolean;
}

export interface LocalKnowledgePolicy {
  enabled?: boolean;
  retention?: LocalKnowledgeRetentionPolicy;
  retrieval?: LocalKnowledgeRetrievalPolicy;
  ingestion?: LocalKnowledgeIngestionPolicy;
}

export interface LocalKnowledgeIngestMessage {
  conversationId: string;
  messageId: string;
  role: "user" | "assistant" | "system";
  kind?: "tool";
  text: string;
  scope?: LocalKnowledgeScope;
}

export interface LocalKnowledgeIngestWorkdirOptions {
  root: string;
  maxFiles?: number;
  maxBytesPerFile?: number;
  scope?: LocalKnowledgeScope;
}

export interface LocalKnowledgeSearchParams {
  query: string;
  limit?: number;
  scope?: LocalKnowledgeScope;
  /** @deprecated Use scope.conversationId. */
  conversationId?: string;
  /** @deprecated Use scope.workdir. */
  workdir?: string;
}

export interface LocalKnowledgeHit {
  id: string;
  sourceType: LocalKnowledgeSourceType;
  sourceUri: string;
  title?: string;
  text: string;
  score?: number;
  updatedAt?: number;
  metadata?: Record<string, unknown>;
}

export interface LocalKnowledgeSearchResult {
  object: "local_knowledge_search_result";
  data: LocalKnowledgeHit[];
}

export interface LocalKnowledgeContextParams extends LocalKnowledgeSearchParams {
  maxBytes?: number;
}

export interface LocalKnowledgeContext {
  hits: LocalKnowledgeHit[];
  text: string;
}

export interface LocalKnowledgeStats {
  object: "local_knowledge_stats";
  sources: number;
  chunks: number;
  bytes: number;
  deletedSources: number;
  oldestIndexedAt?: number;
  newestIndexedAt?: number;
  bySourceType?: Partial<Record<LocalKnowledgeSourceType, {
    sources: number;
    chunks: number;
    bytes: number;
  }>>;
}

export interface LocalKnowledgePruneParams {
  policy?: LocalKnowledgePolicy;
  scope?: LocalKnowledgeScope;
  dryRun?: boolean;
}

export interface LocalKnowledgePruneResult {
  object: "local_knowledge_prune_result";
  dryRun?: boolean;
  deletedSources: number;
  deletedChunks: number;
  reclaimedBytes: number;
}

export interface LocalKnowledgeForgetParams {
  conversationId?: string;
  workspaceId?: string;
  profile?: string;
  workdir?: string;
  sourceUri?: string;
  sourceType?: LocalKnowledgeSourceType;
}

export interface LocalKnowledgeService {
  ingestMessage?(message: LocalKnowledgeIngestMessage): Promise<void> | void;
  ingestWorkdir?(options: LocalKnowledgeIngestWorkdirOptions): Promise<void> | void;
  forgetConversation?(conversationId: string): Promise<void> | void;
  forget?(params: LocalKnowledgeForgetParams): Promise<void> | void;
  prune?(params?: LocalKnowledgePruneParams): Promise<LocalKnowledgePruneResult> | LocalKnowledgePruneResult;
  stats?(scope?: LocalKnowledgeScope): Promise<LocalKnowledgeStats> | LocalKnowledgeStats;
  search(params: LocalKnowledgeSearchParams): Promise<LocalKnowledgeSearchResult>;
  contextForPrompt(params: LocalKnowledgeContextParams): Promise<LocalKnowledgeContext | null>;
  dispose?(): void;
}

export interface LocalKnowledgeToolRegistry {
  readonly toolName: string;
  definitions(): FunctionTool[];
  execute(name: string, args: Record<string, unknown>): Promise<Record<string, unknown>>;
}

export function createLocalKnowledgeToolRegistry(
  service: LocalKnowledgeService,
  toolName = "local_knowledge",
): LocalKnowledgeToolRegistry {
  const definition = localKnowledgeToolDefinition(toolName);
  return {
    toolName,
    definitions: () => [{ ...definition }],
    execute: async (name, args) => {
      if (name !== toolName) throw new Error(`unknown local knowledge tool: ${name}`);
      const action = typeof args.action === "string" ? args.action : "search";
      if (action !== "search") {
        return localKnowledgeErrorResult(action, `unsupported local_knowledge action: ${action}`);
      }
      const query = typeof args.query === "string" ? args.query.trim() : "";
      if (!query) {
        return localKnowledgeErrorResult(action, "query is required");
      }
      const limit = typeof args.limit === "number" ? args.limit : undefined;
      return {
        object: "local_knowledge_result",
        action,
        result: await service.search({ query, limit }),
      };
    },
  };
}

export function localKnowledgeToolDefinition(name = "local_knowledge"): FunctionTool {
  return {
    type: "function",
    name,
    description: [
      "Search the local knowledge index for durable local context from prior transcript messages and indexed workdir files.",
      "Use this when the current task may depend on local project history, prior decisions, repo conventions, or nearby documentation.",
      "The index is local and may be incomplete; use local_workdir to verify exact current file contents before editing.",
    ].join(" "),
    parameters: {
      type: "object",
      properties: {
        action: {
          type: "string",
          enum: ["search"],
          description: "Local knowledge operation.",
        },
        query: {
          type: "string",
          description: "Natural language or keyword query.",
        },
        limit: {
          type: "integer",
          description: "Maximum number of hits.",
        },
      },
      required: ["action", "query"],
      additionalProperties: false,
    },
    strict: false,
  };
}

export function formatLocalKnowledgeContext(context: LocalKnowledgeContext): string {
  if (context.hits.length === 0) return "";
  return [
    "Local knowledge follows. It is retrieved from local transcripts and indexed local files; treat it as contextual hints and verify exact file contents with local_workdir when precision matters.",
    context.text,
  ].filter(Boolean).join("\n");
}

function localKnowledgeErrorResult(action: string, message: string): Record<string, unknown> {
  return {
    object: "local_knowledge_result",
    action,
    error: { message },
  };
}
