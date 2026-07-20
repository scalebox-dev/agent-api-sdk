from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from typing import Any, Literal, Protocol, TypedDict, cast

from agent_api.types.tools import FunctionTool, Tool

LocalKnowledgeSourceType = Literal["transcript", "workdir_file", "note", "tool_output"]


class LocalKnowledgeScope(TypedDict, total=False):
    conversationId: str
    workspaceId: str
    profile: str
    workdir: str
    tags: list[str]


class LocalKnowledgeRetentionPolicy(TypedDict, total=False):
    transcriptTtlSeconds: int
    workdirTtlSeconds: int
    maxBytes: int
    maxTranscriptSources: int
    maxWorkdirSources: int
    deletedTtlSeconds: int


class LocalKnowledgeRetrievalPolicy(TypedDict, total=False):
    defaultLimit: int
    maxLimit: int
    defaultContextBytes: int
    maxContextBytes: int
    scopeMode: Literal["prefer", "filter"]
    includeConversationSiblings: bool


class LocalKnowledgeIngestionPolicy(TypedDict, total=False):
    maxTranscriptBytes: int
    maxWorkdirFiles: int
    maxWorkdirFileBytes: int
    maxChunkBytes: int
    includeWorkdir: bool
    includeTranscripts: bool


class LocalKnowledgePolicy(TypedDict, total=False):
    enabled: bool
    retention: LocalKnowledgeRetentionPolicy
    retrieval: LocalKnowledgeRetrievalPolicy
    ingestion: LocalKnowledgeIngestionPolicy


class LocalKnowledgeHit(TypedDict, total=False):
    id: str
    sourceType: LocalKnowledgeSourceType
    sourceUri: str
    title: str
    text: str
    score: float
    updatedAt: int
    metadata: dict[str, Any]


class LocalKnowledgeSearchResult(TypedDict):
    object: Literal["local_knowledge_search_result"]
    data: list[LocalKnowledgeHit]


class LocalKnowledgeContext(TypedDict):
    hits: list[LocalKnowledgeHit]
    text: str


class LocalKnowledgeSourceStats(TypedDict):
    sources: int
    chunks: int
    bytes: int


class LocalKnowledgeStats(TypedDict, total=False):
    object: Literal["local_knowledge_stats"]
    sources: int
    chunks: int
    bytes: int
    deletedSources: int
    oldestIndexedAt: int
    newestIndexedAt: int
    bySourceType: dict[LocalKnowledgeSourceType, LocalKnowledgeSourceStats]


class LocalKnowledgePruneParams(TypedDict, total=False):
    policy: LocalKnowledgePolicy
    scope: LocalKnowledgeScope
    dryRun: bool


class LocalKnowledgePruneResult(TypedDict, total=False):
    object: Literal["local_knowledge_prune_result"]
    dryRun: bool
    deletedSources: int
    deletedChunks: int
    reclaimedBytes: int


class LocalKnowledgeForgetParams(TypedDict, total=False):
    conversationId: str
    workspaceId: str
    profile: str
    workdir: str
    sourceUri: str
    sourceType: LocalKnowledgeSourceType


class LocalKnowledgeSearchParams(TypedDict, total=False):
    query: str
    limit: int
    scope: LocalKnowledgeScope
    conversationId: str
    workdir: str


class LocalKnowledgeContextParams(LocalKnowledgeSearchParams, total=False):
    maxBytes: int


class LocalKnowledgeIngestMessage(TypedDict, total=False):
    conversationId: str
    messageId: str
    role: Literal["user", "assistant", "system"]
    kind: Literal["tool"]
    text: str
    scope: LocalKnowledgeScope


class LocalKnowledgeIngestWorkdirOptions(TypedDict, total=False):
    root: str
    maxFiles: int
    maxBytesPerFile: int
    scope: LocalKnowledgeScope


class LocalKnowledgeService(Protocol):
    def search(self, params: LocalKnowledgeSearchParams) -> LocalKnowledgeSearchResult: ...

    def context_for_prompt(self, params: LocalKnowledgeContextParams) -> LocalKnowledgeContext | None: ...

    def ingest_message(self, message: LocalKnowledgeIngestMessage) -> None: ...

    def ingest_workdir(self, options: LocalKnowledgeIngestWorkdirOptions) -> None: ...

    def forget_conversation(self, conversation_id: str) -> None: ...

    def forget(self, params: LocalKnowledgeForgetParams) -> None: ...

    def prune(self, params: LocalKnowledgePruneParams | None = None) -> LocalKnowledgePruneResult: ...

    def stats(self, scope: LocalKnowledgeScope | None = None) -> LocalKnowledgeStats: ...

    def dispose(self) -> None: ...


LOCAL_KNOWLEDGE_TOOL_DESCRIPTION = " ".join(
    [
        "Search the local knowledge index for durable local context from prior transcript messages and indexed workdir files.",
        "Use this when the current task may depend on local project history, prior decisions, repo conventions, or nearby documentation.",
        "The index is local and may be incomplete; use local_workdir to verify exact current file contents before editing.",
    ]
)


@dataclass(frozen=True)
class LocalKnowledgeToolRegistry:
    service: LocalKnowledgeService
    tool_name: str = "local_knowledge"

    def definitions(self) -> list[Tool]:
        return [cast(Tool, dict(local_knowledge_tool_definition(self.tool_name)))]

    def execute(self, name: str, args: Mapping[str, Any]) -> dict[str, Any]:
        if name != self.tool_name:
            raise ValueError(f"unknown local knowledge tool: {name}")
        action = args.get("action") if isinstance(args.get("action"), str) else "search"
        if action != "search":
            return _local_knowledge_error_result(str(action), f"unsupported local_knowledge action: {action}")
        query = args.get("query")
        query = query.strip() if isinstance(query, str) else ""
        if not query:
            return _local_knowledge_error_result("search", "query is required")
        limit = args.get("limit") if isinstance(args.get("limit"), int) else None
        params: LocalKnowledgeSearchParams = {"query": query}
        if limit is not None:
            params["limit"] = limit
        return {
            "object": "local_knowledge_result",
            "action": "search",
            "result": self.service.search(params),
        }


def create_local_knowledge_tool_registry(
    service: LocalKnowledgeService,
    *,
    tool_name: str = "local_knowledge",
) -> LocalKnowledgeToolRegistry:
    return LocalKnowledgeToolRegistry(service=service, tool_name=tool_name)


def local_knowledge_tool_definition(name: str = "local_knowledge") -> FunctionTool:
    return {
        "type": "function",
        "name": name,
        "description": LOCAL_KNOWLEDGE_TOOL_DESCRIPTION,
        "parameters": {
            "type": "object",
            "properties": {
                "action": {
                    "type": "string",
                    "enum": ["search"],
                    "description": "Local knowledge operation.",
                },
                "query": {
                    "type": "string",
                    "description": "Natural language or keyword query.",
                },
                "limit": {
                    "type": "integer",
                    "description": "Maximum number of hits.",
                },
            },
            "required": ["action", "query"],
            "additionalProperties": False,
        },
        "strict": False,
    }


def format_local_knowledge_context(context: LocalKnowledgeContext) -> str:
    if not context["hits"]:
        return ""
    return "\n".join(
        [
            "Local knowledge follows. It is retrieved from local transcripts and indexed local files; treat it as contextual hints and verify exact file contents with local_workdir when precision matters.",
            context["text"],
        ]
    )


def _local_knowledge_error_result(action: str, message: str) -> dict[str, Any]:
    return {
        "object": "local_knowledge_result",
        "action": action,
        "error": {"message": message},
    }
