import { isRecord } from "../../shared/runtime-support.js";
/** Encodes one server-sent-event request item. */
export function encodeSSERequestItem(value: unknown): string {
  if (!isRecord(value)) throw new TypeError("SSE stream item must be an object");
  for (const field of Object.keys(value)) {
    if (field !== "data" && field !== "event" && field !== "id" && field !== "retry")
      throw new TypeError(`SSE stream item contains unsupported field ${field}`);
  }
  if (!Object.hasOwn(value, "data") || typeof value["data"] !== "string")
    throw new TypeError("SSE stream item data must be a string");

  const event: string | undefined = sseRequestStringField(value, "event");
  const id: string | undefined = sseRequestStringField(value, "id");
  if (event !== undefined && /[\r\n]/.test(event))
    throw new TypeError("SSE stream item event must not contain a line break");
  if (id !== undefined && /[\u0000\r\n]/.test(id))
    throw new TypeError("SSE stream item id must not contain NUL or a line break");

  let retry: number | undefined;
  if (Object.hasOwn(value, "retry") && value["retry"] !== undefined) {
    if (
      typeof value["retry"] !== "number" ||
      !Number.isSafeInteger(value["retry"]) ||
      value["retry"] < 0
    )
      throw new TypeError("SSE stream item retry must be a non-negative safe integer");
    retry = value["retry"];
  }

  const lines: string[] = [];
  if (event !== undefined) lines.push(`event: ${event}`);
  for (const line of value["data"].split(/\r\n|\r|\n/)) lines.push(`data: ${line}`);
  if (id !== undefined) lines.push(`id: ${id}`);
  if (retry !== undefined) lines.push(`retry: ${retry}`);
  return lines.join("\n") + "\n\n";
}

function sseRequestStringField(
  value: Readonly<Record<string, unknown>>,
  field: "event" | "id",
): string | undefined {
  if (!Object.hasOwn(value, field) || value[field] === undefined) return undefined;
  if (typeof value[field] !== "string")
    throw new TypeError(`SSE stream item ${field} must be a string`);
  return value[field];
}
