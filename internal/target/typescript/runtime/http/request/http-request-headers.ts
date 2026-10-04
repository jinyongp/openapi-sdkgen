const reservedHeaders: ReadonlySet<string> = /* @__PURE__ */ new Set([
  "accept",
  "authorization",
  "content-type",
  "x-csrf-token",
  "x-request-id",
]);

/** Adds consumer headers while rejecting reserved and contract-owned names. */
export function appendRawHeaders(
  target: Headers,
  source: HeadersInit | undefined,
  contractNames: ReadonlySet<string>,
): void {
  if (source === undefined) return;
  const incoming: Headers = new Headers(source);
  incoming.forEach((value: string, name: string): void => {
    const lower: string = name.toLowerCase();
    if (reservedHeaders.has(lower) || contractNames.has(lower)) {
      throw new TypeError(`Raw header ${name} must use its typed option`);
    }
    target.set(name, value);
  });
}

/** Sets a typed header when its value is supplied. */
export function setHeader(headers: Headers, name: string, value: string | undefined): void {
  if (value !== undefined) headers.set(name, value);
}
