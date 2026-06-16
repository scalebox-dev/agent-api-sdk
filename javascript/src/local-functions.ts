import type { FunctionCallOutputInput } from "./types/input.js";
import type { AgentResponse, FunctionCallOutputItem } from "./types/responses.js";

/** Pending client-executed function calls from a requires_action response. */
export function pendingFunctionCalls(response: AgentResponse): FunctionCallOutputItem[] {
  if (response.status !== "requires_action") {
    return [];
  }
  return response.output.filter((item): item is FunctionCallOutputItem => item.type === "function_call");
}

export function functionCallOutputInput(
  callID: string,
  output: string | Record<string, unknown>,
): FunctionCallOutputInput {
  return {
    type: "function_call_output",
    call_id: callID,
    output: typeof output === "string" ? output : JSON.stringify(output),
  };
}

export type LocalFunctionHandler = (args: Record<string, unknown>) => string | Record<string, unknown> | Promise<string | Record<string, unknown>>;

export type LocalFunctionHandlers = Record<string, LocalFunctionHandler>;

/** Run local handlers for pending function calls and return function_call_output input items. */
export async function runLocalFunctionHandlers(
  response: AgentResponse,
  handlers: LocalFunctionHandlers,
): Promise<FunctionCallOutputInput[]> {
  const pending = pendingFunctionCalls(response);
  const outputs: FunctionCallOutputInput[] = [];
  for (const call of pending) {
    const handler = handlers[call.name];
    if (!handler) {
      throw new Error(`no local handler registered for function ${call.name}`);
    }
    let args: Record<string, unknown> = {};
    if (call.arguments) {
      args = JSON.parse(call.arguments) as Record<string, unknown>;
    }
    const result = await handler(args);
    outputs.push(functionCallOutputInput(call.call_id, result));
  }
  return outputs;
}
