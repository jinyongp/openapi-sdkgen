import { describe, expect, it, vi } from "vitest";
import { fragmentedSSE } from "./runtime-quality-helper.js";

describe("SSE fragmentation", () => {
  it("ignores exactly one leading BOM across byte boundaries", async () => {
    expect(await fragmentedSSE(32, 1, "\uFEFFdata: a\n\n")).toEqual([{ data: "a" }]);
    expect(await fragmentedSSE(32, 1, "\uFEFF\uFEFFdata: a\n\ndata: b\n\n")).toEqual([
      { data: "b" },
    ]);
  });
  it.each([64, 128, 256])("scans a fragmented %s KiB line once", async (kib) => {
    const size = kib * 1024;
    await fragmentedSSE(1, 64);
    const scan = vi.spyOn(String.prototype, "charCodeAt");
    try {
      const result = await fragmentedSSE(size, 64);
      const scanned = scan.mock.calls.length;
      scan.mockRestore();
      expect(result).toEqual([{ data: "x".repeat(size) }]);
      expect(scanned).toBeLessThanOrEqual(2 * (size + 16));
    } finally {
      scan.mockRestore();
    }
  });
});
