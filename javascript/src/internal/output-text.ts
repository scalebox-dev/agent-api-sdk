import type { AgentResponse, MessageOutputItem } from "../types/responses.js";

export function addOutputText(response: AgentResponse): AgentResponse {
  if (response.output_text !== undefined) {
    return response;
  }
  response.output_text = response.output
    .filter((item): item is MessageOutputItem => item.type === "message")
    .flatMap((item) => item.content)
    .filter((part) => part.type === "output_text")
    .map((part) => part.text ?? "")
    .join("");
  return response;
}
