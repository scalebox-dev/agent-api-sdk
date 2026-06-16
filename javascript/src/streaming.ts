import { APIConnectionError } from "./errors.js";

export async function* parseSSE<T>(response: Response): AsyncIterable<T> {
  if (!response.body) {
    throw new APIConnectionError("Streaming response did not include a body");
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  for (;;) {
    const { value, done } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    const parts = buffer.split(/\r?\n\r?\n/);
    buffer = parts.pop() ?? "";
    for (const part of parts) {
      const event = parseSSEBlock<T>(part);
      if (event !== undefined) {
        yield event;
      }
    }
  }

  const finalEvent = parseSSEBlock<T>(buffer);
  if (finalEvent !== undefined) {
    yield finalEvent;
  }
}

function parseSSEBlock<T>(block: string): T | undefined {
  const data = block
    .split(/\r?\n/)
    .filter((line) => line.startsWith("data:"))
    .map((line) => line.slice(5).trimStart())
    .join("\n");
  if (!data || data === "[DONE]") {
    return undefined;
  }
  return JSON.parse(data) as T;
}
