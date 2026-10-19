import type { PreferenceStorage } from "./preferences";
import type { SDKTarget } from "../../doc-context";
export interface SDKOptions { goImportPath: string; goPackage: string; server: boolean; metadata: boolean; }
const defaults: SDKOptions = { goImportPath: "example.com/consumer/internal/api", goPackage: "api", server: false, metadata: false };
export function readSDKOptions(storage: PreferenceStorage | undefined, sdk: SDKTarget, query: URLSearchParams): SDKOptions {
  let saved: Partial<SDKOptions> = {};
  try { const value: unknown = JSON.parse(storage?.getItem(`openapi-sdkgen.playground.options.${sdk}.v1`) ?? "null"); if (value && typeof value === "object") saved = value; } catch { /* Defaults remain usable. */ }
  const addons: string[] | null = query.has("with") ? (query.get("with") ?? "").split(",") : null;
  return {
    goImportPath: query.get("go-import-path") ?? (typeof saved.goImportPath === "string" ? saved.goImportPath : defaults.goImportPath),
    goPackage: query.get("go-package") ?? (typeof saved.goPackage === "string" ? saved.goPackage : defaults.goPackage),
    server: addons ? addons.includes("server") : saved.server === true,
    metadata: addons ? addons.includes("metadata") : saved.metadata === true,
  };
}
export function writeSDKOptions(storage: PreferenceStorage | undefined, sdk: SDKTarget, options: SDKOptions): void {
  try { storage?.setItem(`openapi-sdkgen.playground.options.${sdk}.v1`, JSON.stringify(options)); } catch { /* Storage is optional. */ }
}
