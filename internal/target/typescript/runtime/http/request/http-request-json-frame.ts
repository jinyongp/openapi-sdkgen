export function encodeJSONStreamItem(value: unknown): string {
  const encoded: string = JSON.stringify(value);
  if (encoded === undefined) throw new TypeError("stream item is not JSON-serializable");
  return encoded;
}
