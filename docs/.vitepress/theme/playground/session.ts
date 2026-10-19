import { ref, type Ref } from "vue";
import { cancelGeneration, type GeneratedArtifact } from "./wasm";
import type { CodeTheme } from "./highlight";
import type { SDKTarget } from "../../doc-context";
export type AuthenticationMode = "none" | "bearer" | "api-key" | "basic" | "cookies";
const target: Ref<SDKTarget> = ref("typescript");
const generatedTarget: Ref<string> = ref("typescript");
const goImportPath: Ref<string> = ref("example.com/consumer/internal/api");
const goPackage: Ref<string> = ref("api");
const serverAddon: Ref<boolean> = ref(false);
const metadataAddon: Ref<boolean> = ref(false);
const colorTheme: Ref<CodeTheme> = ref<CodeTheme>("github-dark");
const expandedPaths: Ref<ReadonlySet<string>> = ref<ReadonlySet<string>>(new Set());
const url: Ref<string> = ref("");
const localNetworkAccess: Ref<boolean> = ref(false);
const authenticationMode: Ref<AuthenticationMode> = ref<AuthenticationMode>("none");
const bearerToken: Ref<string> = ref("");
const apiKeyHeader: Ref<string> = ref("X-API-Key");
const apiKeyValue: Ref<string> = ref("");
const basicUsername: Ref<string> = ref("");
const basicPassword: Ref<string> = ref("");
const sourceLabel: Ref<string> = ref("");
const artifacts: Ref<GeneratedArtifact[]> = ref<GeneratedArtifact[]>([]);
const selectedPath: Ref<string> = ref("");
const diagnostics: Ref<string> = ref("");
const error: Ref<string> = ref("");
const loading: Ref<boolean> = ref(false);
const dragging: Ref<boolean> = ref(false);
const exporting: Ref<boolean> = ref<boolean>(false);
const copiedPath: Ref<string> = ref<string>("");
const actionError: Ref<string> = ref<string>("");
const source: Ref<string> = ref("");
const exampleID: Ref<string> = ref("");
export interface PlaygroundSession {
  readonly target: Ref<SDKTarget>;
  readonly generatedTarget: Ref<string>;
  readonly goImportPath: Ref<string>;
  readonly goPackage: Ref<string>;
  readonly serverAddon: Ref<boolean>;
  readonly metadataAddon: Ref<boolean>;
  readonly colorTheme: Ref<CodeTheme>;
  readonly expandedPaths: Ref<ReadonlySet<string>>;
  readonly url: Ref<string>;
  readonly localNetworkAccess: Ref<boolean>;
  readonly authenticationMode: Ref<AuthenticationMode>;
  readonly bearerToken: Ref<string>;
  readonly apiKeyHeader: Ref<string>;
  readonly apiKeyValue: Ref<string>;
  readonly basicUsername: Ref<string>;
  readonly basicPassword: Ref<string>;
  readonly sourceLabel: Ref<string>;
  readonly artifacts: Ref<GeneratedArtifact[]>;
  readonly selectedPath: Ref<string>;
  readonly diagnostics: Ref<string>;
  readonly error: Ref<string>;
  readonly loading: Ref<boolean>;
  readonly dragging: Ref<boolean>;
  readonly exporting: Ref<boolean>;
  readonly copiedPath: Ref<string>;
  readonly actionError: Ref<string>;
  readonly source: Ref<string>;
  readonly exampleID: Ref<string>;
}
export const playgroundSession: PlaygroundSession = { target, generatedTarget, goImportPath, goPackage, serverAddon, metadataAddon, colorTheme, expandedPaths, url, localNetworkAccess, authenticationMode, bearerToken, apiKeyHeader, apiKeyValue, basicUsername, basicPassword, sourceLabel, artifacts, selectedPath, diagnostics, error, loading, dragging, exporting, copiedPath, actionError, source, exampleID };
let epoch: number = 0;
let scheduled: ReturnType<typeof setTimeout> | undefined;
let fetching: AbortController | undefined;
export let sessionActive: boolean = false;
export function activateSession(): void { sessionActive = true; }
export function currentEpoch(): number { return epoch; }
export function isCurrent(token: number): boolean { return token === epoch; }
export function invalidateSession(): number {
  epoch++;
  if (scheduled !== undefined) { clearTimeout(scheduled); scheduled = undefined; }
  fetching?.abort(); fetching = undefined;
  cancelGeneration();
  loading.value = false; exporting.value = false;
  return epoch;
}
export function beginFetch(): AbortSignal { fetching = new AbortController(); return fetching.signal; }
export function clearCredentials(): void {
  authenticationMode.value = "none"; bearerToken.value = ""; apiKeyValue.value = "";
  basicUsername.value = ""; basicPassword.value = ""; apiKeyHeader.value = "X-API-Key";
}
export function leaveSession(): void { invalidateSession(); clearCredentials(); url.value = ""; sessionActive = false; }

export function scheduleSessionWork(work: () => void): void { scheduled = setTimeout((): void => { scheduled = undefined; work(); }, 300); }

let contextNavigation: string | undefined;
function navigationPath(href: string): string { return new URL(href, "https://docs.invalid").pathname.replace(/\.html$/, "").replace(/\/$/, ""); }
export function preserveContextNavigation(href: string): void { contextNavigation = navigationPath(href); }
export function consumeContextNavigation(href: string): boolean { const preserve: boolean = contextNavigation === navigationPath(href); contextNavigation = undefined; return preserve; }

let navigationHandler: (() => void) | undefined;
export function registerSessionNavigation(handler: () => void): () => void {
  navigationHandler = handler;
  return (): void => { if (navigationHandler === handler) navigationHandler = undefined; };
}
export function refreshSessionNavigation(): void { navigationHandler?.(); }
