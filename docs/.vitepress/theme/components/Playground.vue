<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from "vue";
import { Check, Copy, FolderDown } from "@lucide/vue";
import { useData, useRoute } from "vitepress";
import CodeViewer from "./CodeViewer.vue";
import FileTree from "./FileTree.vue";
import { playgroundExamples, type PlaygroundExample } from "../playground/examples";
import { codeThemes, type CodeTheme } from "../playground/highlight";
import { readPlaygroundPreferences, writePlaygroundPreferences, readSelectedPath, writeSelectedPath } from "../playground/preferences";
import { buildArtifactTree, findArtifact } from "../playground/tree";
import { generate, type GenerationResult, type GeneratedArtifact } from "../playground/wasm";
import { archiveArtifacts, downloadBlob } from "../playground/download";

const maximumInputBytes = 64 * 1024 * 1024;
const codeThemeValues = codeThemes.map((theme) => theme.value);
const translations = {
  en: {
    heading: "SDK Playground",
    description: "Generate an SDK from an OpenAPI document and inspect the generated types and methods.",
    target: "Target",
    goImportPath: "SDK import path",
    goPackage: "Root package",
    serverAddon: "Server handlers",
    metadataAddon: "OpenAPI metadata",
    generationOnly: "Generates source files only. Compile and run the SDK in your own project.",
    examples: "OpenAPI samples",
    yourDocument: "Your document",
    document: "OpenAPI document",
    dropFile: "Drop a JSON or YAML file",
    chooseFile: "or choose from your computer",
    loadFromURL: "or load from URL",
    urlLabel: "OpenAPI document URL",
    load: "Load",
    authentication: "Authentication",
    authMode: "Authentication method",
    authNone: "None",
    authBearer: "Bearer token",
    authApiKey: "API key header",
    authBasic: "Basic auth",
    authCookies: "Browser cookies",
    bearerToken: "Bearer token",
    apiKeyHeader: "Header name",
    apiKeyValue: "API key",
    basicUsername: "Username",
    basicPassword: "Password",
    cookieHelp: "Uses cookies already stored by this browser. The source server must allow credentialed CORS requests.",
    authPrivacy: "Credentials stay in memory only and are cleared when you change documents or reload the page. Authenticated redirects are blocked.",
    localNetwork: "Allow local network access",
    localNetworkHelp: "May show a browser permission prompt. The source server must still allow CORS.",
    privacy: "Files stay in this browser. URL loading requires CORS. Use a self-contained document; external references and schema extension manifests require local generation.",
    generatedFiles: "Generated files",
    generatedCode: "Generated code",
    colorTheme: "Color theme",
    change: "Change",
    downloadAll: "Download all (ZIP)",
    copyFile: "Copy file",
    copied: "Copied",
    exporting: "Preparing ZIP…",
    exportError: "Could not export generated files. Try again.",
    copyError: "Could not copy this file. Download it instead.",
    generating: "Generating SDK…",
    firstRun: "First run loads the generator into your browser.",
    stopped: "Generation stopped",
    warning: "Generated with warnings",
    emptyTitle: "Generated source appears here",
    emptyDescription: "Choose an example, a file, or a public OpenAPI URL to begin.",
    emptyDocument: "OpenAPI document is empty.",
    tooLarge: "OpenAPI document must be 64 MiB or smaller.",
    noFiles: "Generator returned no files.",
    invalidProtocol: "Use an HTTP or HTTPS URL.",
    invalidURL: "Enter a valid URL.",
    embeddedCredentials: "Keep the document URL and credentials separate. Enter credentials under Authentication.",
    missingBearer: "Enter a bearer token.",
    missingApiKey: "Enter both an API key header name and value.",
    invalidApiKeyHeader: "Enter a valid API key header name.",
    missingBasic: "Enter a Basic auth username or password.",
    authenticatedFetchError: "Could not load this authenticated URL. Check browser network access, CORS, and redirects; authenticated redirects are blocked.",
    corsError: "Could not load this URL. The server may not allow browser CORS requests.",
    localNetworkError: "Could not access the local network. Allow browser access and make sure the source server permits CORS.",
  },
  ko: {
    heading: "SDK 플레이그라운드",
    description: "OpenAPI 문서로 SDK를 생성하고, 생성된 타입과 메서드를 확인하세요.",
    target: "대상",
    goImportPath: "SDK 패키지 경로",
    goPackage: "루트 패키지",
    serverAddon: "서버 핸들러",
    metadataAddon: "OpenAPI 메타데이터",
    generationOnly: "소스 파일만 생성합니다. SDK 컴파일과 실행은 사용자 프로젝트에서 진행하세요.",
    examples: "OpenAPI 예제",
    yourDocument: "내 문서",
    document: "OpenAPI 문서",
    dropFile: "JSON 또는 YAML 파일을 놓으세요",
    chooseFile: "또는 컴퓨터에서 파일 선택",
    loadFromURL: "또는 URL에서 불러오기",
    urlLabel: "OpenAPI 문서 URL",
    load: "불러오기",
    authentication: "인증",
    authMode: "인증 방식",
    authNone: "없음",
    authBearer: "Bearer 토큰",
    authApiKey: "API 키 헤더",
    authBasic: "Basic 인증",
    authCookies: "브라우저 쿠키",
    bearerToken: "Bearer 토큰",
    apiKeyHeader: "헤더 이름",
    apiKeyValue: "API 키",
    basicUsername: "사용자 이름",
    basicPassword: "비밀번호",
    cookieHelp: "이 브라우저에 이미 저장된 쿠키를 사용합니다. 원본 서버에서 인증 정보를 포함한 CORS 요청을 허용해야 합니다.",
    authPrivacy: "인증 정보는 메모리에만 유지되며 문서를 변경하거나 페이지를 새로고침하면 제거됩니다. 인증 요청의 리디렉션은 차단됩니다.",
    localNetwork: "로컬 네트워크 접근 허용",
    localNetworkHelp: "브라우저 권한 요청이 표시될 수 있습니다. 원본 서버의 CORS 허용도 필요합니다.",
    privacy: "파일은 이 브라우저 안에서만 처리됩니다. URL은 CORS 접근이 필요합니다. 외부 참조 없는 문서를 사용하세요. 외부 참조와 스키마 확장 매니페스트는 로컬에서 생성합니다.",
    generatedFiles: "생성된 파일",
    generatedCode: "생성된 코드",
    colorTheme: "색상 테마",
    change: "변경",
    downloadAll: "전체 ZIP 다운로드",
    copyFile: "파일 복사",
    copied: "복사됨",
    exporting: "ZIP 준비 중…",
    exportError: "생성된 파일을 내보내지 못했습니다. 다시 시도하세요.",
    copyError: "파일을 복사하지 못했습니다. 파일 다운로드를 사용하세요.",
    generating: "SDK 생성 중…",
    firstRun: "처음 실행할 때 브라우저에 생성기를 불러옵니다.",
    stopped: "생성 중단",
    warning: "경고와 함께 생성됨",
    emptyTitle: "생성된 소스가 여기에 표시됩니다",
    emptyDescription: "예제를 선택하거나 파일 또는 공개 OpenAPI URL을 사용하세요.",
    emptyDocument: "OpenAPI 문서가 비어 있습니다.",
    tooLarge: "OpenAPI 문서는 64 MiB 이하여야 합니다.",
    noFiles: "생성된 파일이 없습니다.",
    invalidProtocol: "HTTP 또는 HTTPS URL을 사용하세요.",
    invalidURL: "올바른 URL을 입력하세요.",
    embeddedCredentials: "문서 URL과 인증 정보를 분리하고, 인증 정보는 인증 항목에 입력하세요.",
    missingBearer: "Bearer 토큰을 입력하세요.",
    missingApiKey: "API 키 헤더 이름과 값을 모두 입력하세요.",
    invalidApiKeyHeader: "올바른 API 키 헤더 이름을 입력하세요.",
    missingBasic: "Basic 인증 사용자 이름 또는 비밀번호를 입력하세요.",
    authenticatedFetchError: "인증된 URL을 불러올 수 없습니다. 브라우저 네트워크 접근, CORS, 리디렉션 여부를 확인하세요. 인증 요청의 리디렉션은 차단됩니다.",
    corsError: "URL을 불러올 수 없습니다. 서버에서 브라우저 CORS 요청을 허용하지 않을 수 있습니다.",
    localNetworkError: "로컬 네트워크에 접근할 수 없습니다. 브라우저 권한과 원본 서버의 CORS 설정을 확인하세요.",
  },
} as const;

import { parseDocContext, type SDKTarget } from "../../doc-context";
import { playgroundSession, sessionActive, activateSession, scheduleSessionWork, consumeContextNavigation, registerSessionNavigation, currentEpoch, invalidateSession, isCurrent, beginFetch, clearCredentials, leaveSession, type AuthenticationMode } from "../playground/session";
import { readSDKOptions, writeSDKOptions } from "../playground/options";
const { lang } = useData();
const route: ReturnType<typeof useRoute> = useRoute();
const copy = computed(() => lang.value === "ko-KR" ? translations.ko : translations.en);
const { target, generatedTarget, goImportPath, goPackage, serverAddon, metadataAddon, colorTheme, expandedPaths, url, localNetworkAccess, authenticationMode, bearerToken, apiKeyHeader, apiKeyValue, basicUsername, basicPassword, sourceLabel, artifacts, selectedPath, diagnostics, error, loading, dragging, exporting, copiedPath, actionError, source, exampleID } = playgroundSession;
const ready: Ref<boolean> = ref(false);
const fileInput: Ref<HTMLInputElement | undefined> = ref<HTMLInputElement>();
let unregisterNavigation: (() => void) | undefined;
let copyTimer: ReturnType<typeof setTimeout> | undefined;

const selectedArtifact = computed(() => findArtifact(artifacts.value, selectedPath.value));
const selectedLineCount = computed(() => selectedArtifact.value?.content.split("\n").length ?? 0);
const lineCountLabel = computed(() => lang.value === "ko-KR" ? `${selectedLineCount.value}줄` : `${selectedLineCount.value} lines`);
const loaded = computed(() => artifacts.value.length > 0);
const fileCountLabel = computed<string>((): string => lang.value === "ko-KR"
  ? `${artifacts.value.length}개 파일`
  : `${artifacts.value.length} files`);
const tree = computed(() => buildArtifactTree(artifacts.value));
let preferencesReady = false;

function initializeSession(): void {
  preferencesReady = false;
  const previousOptions: string = JSON.stringify([goImportPath.value, goPackage.value, serverAddon.value, metadataAddon.value]);
  const preserveOptions: boolean = consumeContextNavigation(window.location.href);
  const parameters: URLSearchParams = new URLSearchParams(window.location.search);
  const nextTarget: SDKTarget = parseDocContext(route.path)?.sdk ?? (parameters.get("target") === "golang" ? "golang" : "typescript");
  const changed: boolean = target.value !== nextTarget;
  const wasActive: boolean = sessionActive;
  if (changed) invalidateSession();
  const preferences = readPlaygroundPreferences(preferenceStorage(), codeThemeValues, nextTarget);
  if (preferences.codeTheme) colorTheme.value = preferences.codeTheme;
  expandedPaths.value = new Set(preferences.expandedPaths);
  if (changed || !wasActive || !preserveOptions) {
    const options = readSDKOptions(preferenceStorage(), nextTarget, wasActive && preserveOptions ? new URLSearchParams() : parameters);
    goImportPath.value = options.goImportPath; goPackage.value = options.goPackage;
    serverAddon.value = options.server; metadataAddon.value = options.metadata;
  }
  target.value = nextTarget;
  const locationURL: URL = new URL(window.location.href);
  updateGenerationOptions(locationURL);
  window.history.replaceState(window.history.state, "", locationURL);
  writeSDKOptions(preferenceStorage(), nextTarget, { goImportPath: goImportPath.value, goPackage: goPackage.value, server: serverAddon.value, metadata: metadataAddon.value });
  activateSession(); preferencesReady = true; ready.value = true;
  const example: PlaygroundExample | undefined = playgroundExamples.find((candidate: PlaygroundExample): boolean => candidate.id === parameters.get("example"));
  if (example && (!wasActive || exampleID.value !== example.id)) void loadExample(example);
  else if (source.value && (changed || previousOptions !== JSON.stringify([goImportPath.value, goPackage.value, serverAddon.value, metadataAddon.value]) || !artifacts.value.length && !loading.value)) void runGeneration(source.value, sourceLabel.value);
}
function restoreHistory(): void { if (parseDocContext(window.location.href)?.sdk === target.value) initializeSession(); }
onMounted((): void => { initializeSession(); unregisterNavigation = registerSessionNavigation(initializeSession); window.addEventListener("popstate", restoreHistory); });

watch([colorTheme, expandedPaths], () => {
  if (!preferencesReady) return;
  writePlaygroundPreferences(preferenceStorage(), {
    codeTheme: colorTheme.value,
    expandedPaths: expandedPaths.value,
  }, target.value);
});

watch(selectedPath, (): void => {
  copiedPath.value = "";
  actionError.value = "";
  if (selectedPath.value) writeSelectedPath(preferenceStorage(), target.value, selectedPath.value);
});

onBeforeUnmount((): void => {
  if (copyTimer !== undefined) clearTimeout(copyTimer);
  window.removeEventListener("popstate", restoreHistory);
  unregisterNavigation?.();
  if (parseDocContext(route.path)?.topic !== "playground") leaveSession();
});

async function downloadAll(): Promise<void> {
  if (exporting.value || loading.value || !loaded.value) return;
  const token: number = currentEpoch();
  const snapshot: GeneratedArtifact[] = artifacts.value;
  const sdk: string = generatedTarget.value;
  exporting.value = true;
  actionError.value = "";
  try {
    const archive: Uint8Array<ArrayBuffer> = await archiveArtifacts(snapshot);
    if (!isCurrent(token) || snapshot !== artifacts.value || loading.value) return;
    downloadBlob(new Blob([archive], { type: "application/zip" }), `openapi-sdkgen-${sdk}.zip`);
  } catch {
    if (isCurrent(token)) actionError.value = copy.value.exportError;
  } finally {
    if (isCurrent(token)) exporting.value = false;
  }
}

async function copyFile(): Promise<void> {
  const token: number = currentEpoch();
  const artifact: GeneratedArtifact | undefined = selectedArtifact.value;
  if (!artifact) return;
  actionError.value = "";
  try {
    await navigator.clipboard.writeText(artifact.content);
    if (!isCurrent(token) || selectedPath.value !== artifact.path) return;
    copiedPath.value = artifact.path;
    if (copyTimer !== undefined) clearTimeout(copyTimer);
    copyTimer = setTimeout((): void => { copiedPath.value = ""; }, 2000);
  } catch {
    if (isCurrent(token)) actionError.value = copy.value.copyError;
  }
}

function preferenceStorage(): Storage | undefined {
  try {
    return window.localStorage;
  } catch {
    return undefined;
  }
}

type TargetAddressSpace = "local" | "loopback";
type LocalNetworkRequestInit = RequestInit & { targetAddressSpace?: TargetAddressSpace };

function encodeBasicCredential(username: string, password: string): string {
  const bytes = new TextEncoder().encode(`${username}:${password}`);
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

function applyAuthentication(request: LocalNetworkRequestInit): void {
  if (authenticationMode.value === "none") return;
  request.redirect = "error";
  if (authenticationMode.value === "cookies") {
    request.credentials = "include";
    return;
  }

  const headers = new Headers();
  if (authenticationMode.value === "bearer") {
    const token = bearerToken.value.trim();
    if (token === "") throw new Error(copy.value.missingBearer);
    headers.set("Authorization", `Bearer ${token}`);
  } else if (authenticationMode.value === "api-key") {
    const name = apiKeyHeader.value.trim();
    if (name === "" || apiKeyValue.value === "") throw new Error(copy.value.missingApiKey);
    try {
      headers.set(name, apiKeyValue.value);
    } catch {
      throw new Error(copy.value.invalidApiKeyHeader);
    }
  } else {
    if (basicUsername.value === "" && basicPassword.value === "")
      throw new Error(copy.value.missingBasic);
    headers.set("Authorization", `Basic ${encodeBasicCredential(basicUsername.value, basicPassword.value)}`);
  }
  request.headers = headers;
}

function targetAddressSpace(url: URL): TargetAddressSpace {
  const hostname = url.hostname.toLowerCase().replace(/^\[|\]$/g, "");
  if (
    hostname === "localhost" ||
    hostname.endsWith(".localhost") ||
    hostname === "::1" ||
    hostname.startsWith("127.")
  ) {
    return "loopback";
  }
  return "local";
}

function resetMessages() {
  error.value = "";
  diagnostics.value = "";
}

function validateSize(size: number) {
  if (size === 0) throw new Error(copy.value.emptyDocument);
  if (size > maximumInputBytes) throw new Error(copy.value.tooLarge);
}

function updateGenerationOptions(locationURL: URL): void {
  for (const key of [...locationURL.searchParams.keys()]) {
    if (!["example", "with", "go-import-path", "go-package"].includes(key)) locationURL.searchParams.delete(key);
  }
  if (!playgroundExamples.some((example: PlaygroundExample): boolean => example.id === locationURL.searchParams.get("example"))) locationURL.searchParams.delete("example");
  if (target.value === "golang") {
    locationURL.searchParams.delete("target");
    locationURL.searchParams.set("go-import-path", goImportPath.value);
    locationURL.searchParams.set("go-package", goPackage.value);
  } else {
    for (const key of ["target", "go-import-path", "go-package"]) locationURL.searchParams.delete(key);
  }
  const addons: string[] = [];
  if (serverAddon.value) addons.push("server");
  if (metadataAddon.value) addons.push("metadata");
  if (addons.length) locationURL.searchParams.set("with", addons.join(","));
  else locationURL.searchParams.set("with", "");
}

async function runGeneration(contents: string, label: string): Promise<void> {
  const token: number = invalidateSession();
  source.value = contents; sourceLabel.value = label;
  const locationURL: URL = new URL(window.location.href);
  updateGenerationOptions(locationURL);
  window.history.replaceState(window.history.state, "", locationURL);
  resetMessages(); artifacts.value = []; selectedPath.value = ""; loading.value = true;
  const selectedTarget: SDKTarget = target.value;
  writeSDKOptions(preferenceStorage(), selectedTarget, { goImportPath: goImportPath.value, goPackage: goPackage.value, server: serverAddon.value, metadata: metadataAddon.value });
  try {
    validateSize(new TextEncoder().encode(contents).byteLength);
    if (selectedTarget === "golang" && !goImportPath.value.trim()) throw new Error(lang.value === "ko-KR" ? "Go 패키지 경로를 입력하세요." : "Enter a Go import path.");
    const addons: string[] = [];
    if (serverAddon.value) addons.push("server"); if (metadataAddon.value) addons.push("metadata");
    const result: GenerationResult = await generate(contents, selectedTarget, { addons, ...(selectedTarget === "golang" ? { goImportPath: goImportPath.value, goPackage: goPackage.value } : {}) });
    if (!isCurrent(token)) return;
    generatedTarget.value = selectedTarget;
    if (result.error) throw new Error(result.error);
    diagnostics.value = result.diagnostics ?? "";
    if (!result.artifacts?.length) { if (!diagnostics.value) throw new Error(copy.value.noFiles); return; }
    artifacts.value = result.artifacts;
    selectedPath.value = readSelectedPath(preferenceStorage(), selectedTarget, result.artifacts.map((artifact: GeneratedArtifact): string => artifact.path));
  } catch (cause) {
    if (isCurrent(token)) error.value = cause instanceof Error ? cause.message : String(cause);
  } finally { if (isCurrent(token)) loading.value = false; }
}

watch([goImportPath, goPackage, serverAddon, metadataAddon], (): void => {
  if (!preferencesReady) return;
  writeSDKOptions(preferenceStorage(), target.value, { goImportPath: goImportPath.value, goPackage: goPackage.value, server: serverAddon.value, metadata: metadataAddon.value });
  if (source.value) {
    invalidateSession(); artifacts.value = []; loading.value = true;
    scheduleSessionWork((): void => { void runGeneration(source.value, sourceLabel.value); });
  }
}, { flush: "sync" });

async function loadFile(file?: File): Promise<void> {
  if (!file) return;
  const token: number = invalidateSession();
  loading.value = true;
  try {
    validateSize(file.size);
    const contents: string = await file.text();
    if (!isCurrent(token)) return;
    clearCredentials(); exampleID.value = "";
    const locationURL: URL = new URL(window.location.href); locationURL.searchParams.delete("example");
    window.history.replaceState(window.history.state, "", locationURL);
    await runGeneration(contents, file.name);
  } catch (cause) { if (isCurrent(token)) error.value = cause instanceof Error ? cause.message : String(cause); }
  finally { if (isCurrent(token)) loading.value = false; }
}

async function loadURL(): Promise<void> {
  const token: number = invalidateSession();
  resetMessages();
  let parsed: URL;
  try {
    parsed = new URL(url.value);
  } catch {
    error.value = copy.value.invalidURL;
    return;
  }
  if (parsed.protocol !== "https:" && parsed.protocol !== "http:") {
    error.value = copy.value.invalidProtocol;
    return;
  }
  if (parsed.username !== "" || parsed.password !== "") {
    error.value = copy.value.embeddedCredentials;
    return;
  }

  loading.value = true;
  try {
    const request: LocalNetworkRequestInit = { mode: "cors", signal: beginFetch() };
    if (localNetworkAccess.value) request.targetAddressSpace = targetAddressSpace(parsed);
    applyAuthentication(request);
    const response = await fetch(parsed, request);
    if (!response.ok) {
      const message = lang.value === "ko-KR"
        ? `문서를 불러올 수 없습니다 (${response.status}).`
        : `Could not load the document (${response.status}).`;
      throw new Error(message);
    }
    const length = Number(response.headers.get("content-length"));
    if (Number.isFinite(length) && length > 0) validateSize(length);
    const contents: string = await response.text();
    if (!isCurrent(token)) return;
    const label: string = `${parsed.hostname}${parsed.pathname}`;
    exampleID.value = "";
    const locationURL: URL = new URL(window.location.href); locationURL.searchParams.delete("example");
    window.history.replaceState(window.history.state, "", locationURL);
    await runGeneration(contents, label);
  } catch (cause) {
    if (!isCurrent(token)) return;
    error.value = cause instanceof TypeError
      ? authenticationMode.value !== "none"
        ? copy.value.authenticatedFetchError
        : localNetworkAccess.value
          ? copy.value.localNetworkError
          : copy.value.corsError
      : cause instanceof Error ? cause.message : String(cause);
    loading.value = false;
  }
}

function handleDrop(event: DragEvent) {
  dragging.value = false;
  void loadFile(event.dataTransfer?.files[0]);
}

function exampleTitle(example: PlaygroundExample): string {
  return lang.value === "ko-KR" ? example.title.ko : example.title.en;
}

function exampleDescription(example: PlaygroundExample): string {
  return lang.value === "ko-KR" ? example.description.ko : example.description.en;
}

async function loadExample(example: PlaygroundExample): Promise<void> {
  clearCredentials();
  exampleID.value = example.id;
  const locationURL = new URL(window.location.href);
  locationURL.searchParams.set("example", example.id);
  window.history.replaceState(window.history.state, "", locationURL);

  await runGeneration(
    example.document,
    `${exampleTitle(example)} · OpenAPI ${example.version}`,
  );
}

function startOver(): void {
  invalidateSession();
  source.value = ""; exampleID.value = "";
  const locationURL = new URL(window.location.href);
  locationURL.searchParams.delete("example");
  window.history.replaceState(window.history.state, "", locationURL);
  sourceLabel.value = "";
  artifacts.value = [];
  selectedPath.value = "";
  diagnostics.value = "";
  error.value = "";
  actionError.value = "";
  copiedPath.value = "";
  url.value = "";
  authenticationMode.value = "none";
  bearerToken.value = "";
  apiKeyHeader.value = "X-API-Key";
  apiKeyValue.value = "";
  basicUsername.value = "";
  basicPassword.value = "";
  if (fileInput.value) fileInput.value.value = "";
}

function toggleDirectory(path: string) {
  const next = new Set(expandedPaths.value);
  if (next.has(path)) next.delete(path);
  else next.add(path);
  expandedPaths.value = next;
}
</script>

<template>
  <div class="sdk-playground">
    <header class="playground-heading">
      <div>
        <h1>{{ copy.heading }}</h1>
        <p>{{ copy.description }}</p>
      </div>
    </header>

    <p class="visually-hidden" role="status">{{ loading ? copy.generating : error ? copy.stopped : loaded ? `${generatedTarget === 'golang' ? 'Go' : 'TypeScript'} · ${fileCountLabel}` : '' }}</p>

    <section class="workbench" :class="{ 'has-files': loaded }">
      <aside class="source-panel">
        <div class="source-controls" :class="{ compact: loaded }">
          <p class="field-label">SDK · {{ target === 'golang' ? 'Go' : 'TypeScript' }}</p>
          <template v-if="target === 'golang'">
            <label class="field-label" for="playground-go-import-path">{{ copy.goImportPath }}</label>
            <input id="playground-go-import-path" v-model="goImportPath" type="text" :disabled="!ready" :aria-describedby="error ? 'playground-generation-error' : undefined" :aria-invalid="!goImportPath.trim() || !!error && /Go import path|--go-import-path/.test(error)" spellcheck="false" />
            <label class="field-label" for="playground-go-package">{{ copy.goPackage }}</label>
            <input id="playground-go-package" v-model="goPackage" type="text" :disabled="!ready" :aria-describedby="error ? 'playground-generation-error' : undefined" :aria-invalid="!!error && /invalid Go package/.test(error)" spellcheck="false" />
          </template>
          <label><input v-model="serverAddon" type="checkbox" :disabled="!ready" /> {{ copy.serverAddon }}</label>
          <label><input v-model="metadataAddon" type="checkbox" :disabled="!ready" /> {{ copy.metadataAddon }}</label>
          <p class="field-help">{{ copy.generationOnly }}</p>
          <button v-if="loading" type="button" @click="startOver">{{ copy.change }}</button>

          <template v-if="!loaded">
          <p class="field-label input-label">{{ copy.examples }}</p>
          <div class="example-list">
            <button
              v-for="example in playgroundExamples"
              :key="example.id"
              class="example-item"
              type="button"
              :disabled="loading || !ready"
              @click="loadExample(example)"
            >
              <span class="example-heading">
                <strong>{{ exampleTitle(example) }}</strong>
                <small>OpenAPI {{ example.version }}</small>
              </span>
              <span>{{ exampleDescription(example) }}</span>
            </button>
          </div>

          <div class="separator source-separator"><span>{{ copy.yourDocument }}</span></div>
          <p class="field-label">{{ copy.document }}</p>
          <button
            class="drop-zone"
            :class="{ dragging }"
            type="button"
            :disabled="loading || !ready"
            @click="fileInput?.click()"
            @dragenter.prevent="dragging = true"
            @dragover.prevent
            @dragleave.prevent="dragging = false"
            @drop.prevent="handleDrop"
          >
            <span class="upload-icon" aria-hidden="true">↑</span>
            <strong>{{ copy.dropFile }}</strong>
            <span>{{ copy.chooseFile }}</span>
          </button>
          <input
            ref="fileInput"
            class="visually-hidden"
            type="file"
            accept=".json,.yaml,.yml,application/json,application/yaml,text/yaml"
            @change="loadFile(($event.target as HTMLInputElement).files?.[0])"
          />

          <div class="separator"><span>{{ copy.loadFromURL }}</span></div>
          <form class="url-form" @submit.prevent="loadURL">
            <input v-model.trim="url" type="url" placeholder="https://example.com/openapi.yaml" :aria-label="copy.urlLabel" />
            <button type="submit" :disabled="loading || !url">{{ copy.load }}</button>
          </form>
          <details class="auth-panel">
            <summary>{{ copy.authentication }}</summary>
            <div class="auth-fields">
              <label>
                <span>{{ copy.authMode }}</span>
                <select v-model="authenticationMode">
                  <option value="none">{{ copy.authNone }}</option>
                  <option value="bearer">{{ copy.authBearer }}</option>
                  <option value="api-key">{{ copy.authApiKey }}</option>
                  <option value="basic">{{ copy.authBasic }}</option>
                  <option value="cookies">{{ copy.authCookies }}</option>
                </select>
              </label>
              <label v-if="authenticationMode === 'bearer'">
                <span>{{ copy.bearerToken }}</span>
                <input v-model="bearerToken" type="password" autocomplete="off" spellcheck="false" />
              </label>
              <template v-else-if="authenticationMode === 'api-key'">
                <label>
                  <span>{{ copy.apiKeyHeader }}</span>
                  <input v-model="apiKeyHeader" type="text" autocomplete="off" spellcheck="false" />
                </label>
                <label>
                  <span>{{ copy.apiKeyValue }}</span>
                  <input v-model="apiKeyValue" type="password" autocomplete="off" spellcheck="false" />
                </label>
              </template>
              <template v-else-if="authenticationMode === 'basic'">
                <label>
                  <span>{{ copy.basicUsername }}</span>
                  <input v-model="basicUsername" type="text" autocomplete="off" spellcheck="false" />
                </label>
                <label>
                  <span>{{ copy.basicPassword }}</span>
                  <input v-model="basicPassword" type="password" autocomplete="off" />
                </label>
              </template>
              <p v-else-if="authenticationMode === 'cookies'" class="auth-help">{{ copy.cookieHelp }}</p>
              <p class="auth-help">{{ copy.authPrivacy }}</p>
            </div>
          </details>
          <label class="network-option">
            <input v-model="localNetworkAccess" type="checkbox" />
            <span>
              <strong>{{ copy.localNetwork }}</strong>
              <small>{{ copy.localNetworkHelp }}</small>
            </span>
          </label>
          <p class="privacy-note">{{ copy.privacy }}</p>
          </template>
        </div>

        <template v-if="loaded">
          <div class="tree-header">
            <div>
              <span class="field-label">{{ copy.generatedFiles }}</span>
              <strong :title="sourceLabel">{{ sourceLabel }}</strong>
            </div>
            <button type="button" :disabled="exporting" @click="startOver">{{ copy.change }}</button>
          </div>
          <div class="target-summary"><span>{{ fileCountLabel }}</span><strong>{{ generatedTarget === "golang" ? "Go" : "TypeScript" }}</strong></div>
          <nav class="tree-scroll" :aria-label="copy.generatedFiles">
            <FileTree
              :expanded-paths="expandedPaths"
              :nodes="tree"
              :selected-path="selectedPath"
              @select="selectedPath = $event"
              @toggle="toggleDirectory"
            />
          </nav>
        </template>
      </aside>

      <section class="code-panel">
        <div v-if="loading" class="empty-code loading-state" aria-live="polite">
          <span class="spinner" aria-hidden="true"></span>
          <strong>{{ copy.generating }}</strong>
          <span>{{ copy.firstRun }}</span>
        </div>
        <div v-else-if="error || (diagnostics && !loaded)" class="empty-code error-state" aria-live="polite">
          <span class="error-icon" aria-hidden="true">!</span>
          <strong>{{ copy.stopped }}</strong>
          <pre id="playground-generation-error">{{ error || diagnostics }}</pre>
          <button v-if="source" type="button" @click="runGeneration(source, sourceLabel)">{{ lang === 'ko-KR' ? '다시 생성' : 'Retry generation' }}</button>
        </div>
        <template v-else-if="selectedArtifact">
          <header class="code-header">
            <div class="file-title"><span class="file-dot"></span><strong :title="selectedArtifact.path">{{ selectedArtifact.path }}</strong></div>
            <div class="code-meta">
              <span>{{ lineCountLabel }}</span>
              <label class="theme-picker">
                <span class="visually-hidden">{{ copy.colorTheme }}</span>
                <select v-model="colorTheme" :aria-label="copy.colorTheme">
                  <option v-for="themeOption in codeThemes" :key="themeOption.value" :value="themeOption.value">
                    {{ themeOption.label }}
                  </option>
                </select>
              </label>
              <button class="code-action" type="button" :title="copiedPath === selectedPath ? copy.copied : copy.copyFile" :aria-label="copiedPath === selectedPath ? copy.copied : copy.copyFile" @click="copyFile">
                <Check v-if="copiedPath === selectedPath" :size="16" aria-hidden="true" />
                <Copy v-else :size="16" aria-hidden="true" />
              </button>
              <button class="download-all" type="button" :disabled="exporting || loading" :aria-busy="exporting" @click="downloadAll">
                <FolderDown :size="16" aria-hidden="true" />
                {{ exporting ? copy.exporting : copy.downloadAll }}
              </button>
            </div>
          </header>
          <p v-if="actionError" class="action-error" role="alert">{{ actionError }}</p>
          <span class="visually-hidden" role="status">{{ copiedPath ? copy.copied : '' }}</span>
          <details v-if="diagnostics" class="diagnostic-banner">
            <summary>{{ copy.warning }}</summary>
            <pre>{{ diagnostics }}</pre>
          </details>
          <CodeViewer
            :content="selectedArtifact.content"
            :path="selectedArtifact.path"
            :theme="colorTheme"
            :aria-label="copy.generatedCode"
          />
        </template>
        <div v-else class="empty-code">
          <span class="code-placeholder" aria-hidden="true">&lt;/&gt;</span>
          <strong>{{ copy.emptyTitle }}</strong>
          <span>{{ copy.emptyDescription }}</span>
        </div>
      </section>
    </section>
  </div>
</template>

<style scoped>
.sdk-playground {
  width: min(1440px, calc(100vw - 48px));
  margin: 0 0 0 50%;
  padding-top: 48px;
  transform: translateX(-50%);
  color: var(--vp-c-text-1);
}

.playground-heading { display: flex; align-items: flex-end; justify-content: space-between; gap: 24px; margin-bottom: 28px; }
.playground-heading > div { min-width: 0; max-width: 100%; }
.playground-heading h1, .playground-heading p { overflow-wrap: anywhere; }
.playground-heading h1 { margin: 2px 0 6px; border: 0; font-size: 32px; line-height: 1.15; letter-spacing: 0; }
.playground-heading p { margin: 0; color: var(--vp-c-text-2); }
.workbench { display: grid; height: min(680px, calc(100dvh - 220px)); max-height: calc(100dvh - 96px); grid-template-columns: minmax(280px, 360px) minmax(0, 1fr); overflow: hidden; border: 1px solid var(--vp-c-divider); border-radius: 16px; background: var(--vp-c-bg); box-shadow: 0 22px 65px rgba(15, 23, 42, .08); }
.source-panel { display: flex; min-height: 0; overflow: hidden; flex-direction: column; border-right: 1px solid var(--vp-c-divider); background: color-mix(in srgb, var(--vp-c-bg-soft) 70%, var(--vp-c-bg)); }
.source-controls.compact { flex: 0 0 auto; max-height: 45%; padding: 16px; }
.source-controls { overflow: auto; padding: 26px; }
.field-label { display: block; margin-bottom: 8px; color: var(--vp-c-text-2); font-size: 12px; font-weight: 750; letter-spacing: .06em; text-transform: uppercase; }
.input-label { margin-top: 24px; }
select, .url-form input, .auth-fields input, .source-controls > input[type="text"] { width: 100%; height: 42px; border: 1px solid var(--vp-c-divider); border-radius: 9px; color: var(--vp-c-text-1); background: var(--vp-c-bg); font: inherit; }
select { padding: 0 12px; }
.source-controls > input[type="text"] { padding: 0 11px; margin-bottom: 12px; min-width: 0; }
.example-list { display: grid; gap: 8px; }
.example-item { display: grid; width: 100%; gap: 4px; padding: 11px 12px; border: 1px solid var(--vp-c-divider); border-radius: 10px; color: var(--vp-c-text-2); background: var(--vp-c-bg); cursor: pointer; font: inherit; text-align: left; transition: .15s ease; }
.example-item:hover { border-color: var(--vp-c-brand-1); background: color-mix(in srgb, var(--vp-c-brand-1) 5%, var(--vp-c-bg)); }
.example-item:disabled { opacity: .55; cursor: wait; }
.example-item > span:last-child { font-size: 11px; line-height: 1.4; }
.example-heading { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; }
.example-heading strong { color: var(--vp-c-text-1); font-size: 13px; }
.example-heading small { flex: 0 0 auto; color: var(--vp-c-brand-1); font-size: 10px; font-weight: 700; }
.source-separator { margin-top: 24px; }
.drop-zone { display: flex; width: 100%; min-height: 185px; align-items: center; justify-content: center; flex-direction: column; gap: 6px; border: 1.5px dashed var(--vp-c-divider); border-radius: 12px; color: var(--vp-c-text-2); background: var(--vp-c-bg); cursor: pointer; transition: .15s ease; }
.drop-zone:hover, .drop-zone.dragging { border-color: var(--vp-c-brand-1); background: color-mix(in srgb, var(--vp-c-brand-1) 6%, var(--vp-c-bg)); }
.drop-zone:disabled { opacity: .55; cursor: wait; }
.drop-zone strong { color: var(--vp-c-text-1); font-size: 14px; }
.drop-zone span:last-child { font-size: 12px; }
.upload-icon { display: grid; width: 38px; height: 38px; margin-bottom: 5px; place-items: center; border-radius: 10px; color: var(--vp-c-brand-1); background: color-mix(in srgb, var(--vp-c-brand-1) 10%, transparent); font-size: 21px; }
.separator { display: flex; align-items: center; gap: 10px; margin: 22px 0 12px; color: var(--vp-c-text-2); font-size: 11px; text-transform: uppercase; }
.separator::before, .separator::after { content: ""; flex: 1; height: 1px; background: var(--vp-c-divider); }
.url-form { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; }
.url-form input { padding: 0 11px; min-width: 0; font-size: 13px; }
.auth-panel { margin-top: 14px; border: 1px solid var(--vp-c-divider); border-radius: 10px; background: var(--vp-c-bg); }
.auth-panel summary { padding: 11px 12px; color: var(--vp-c-text-2); font-size: 12px; font-weight: 700; cursor: pointer; user-select: none; }
.auth-panel[open] summary { border-bottom: 1px solid var(--vp-c-divider); }
.auth-fields { display: grid; gap: 12px; padding: 13px 12px; }
.auth-fields label { display: grid; gap: 6px; color: var(--vp-c-text-2); font-size: 11px; font-weight: 650; }
.auth-fields input { padding: 0 11px; min-width: 0; font-size: 13px; font-weight: 400; }
.auth-help { margin: 0; color: var(--vp-c-text-2); font-size: 10.5px; line-height: 1.45; }
.url-form button, .tree-header button { border: 0; border-radius: 9px; font-weight: 700; cursor: pointer; }
.url-form button { padding: 0 15px; color: white; background: var(--vp-c-brand-1); }
.url-form button:disabled { opacity: .5; cursor: not-allowed; }
.network-option { display: flex; align-items: flex-start; gap: 9px; margin-top: 14px; color: var(--vp-c-text-2); cursor: pointer; }
.network-option input { width: 16px; height: 16px; margin: 2px 0 0; accent-color: var(--vp-c-brand-1); }
.network-option span { display: flex; min-width: 0; flex-direction: column; gap: 2px; }
.network-option strong { color: var(--vp-c-text-1); font-size: 12px; font-weight: 650; }
.network-option small { color: var(--vp-c-text-2); font-size: 10.5px; line-height: 1.45; }
.privacy-note { margin: 12px 0 0; color: var(--vp-c-text-2); font-size: 11px; line-height: 1.5; }
.visually-hidden { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; }

.tree-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 22px 20px 16px; border-bottom: 1px solid var(--vp-c-divider); }
.tree-header .field-label { margin: 0 0 3px; }
.tree-header strong { display: block; max-width: 210px; overflow: hidden; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.tree-header button { padding: 7px 10px; color: var(--vp-c-brand-1); background: color-mix(in srgb, var(--vp-c-brand-1) 10%, transparent); font-size: 12px; }
.target-summary { display: flex; justify-content: space-between; padding: 12px 20px; border-bottom: 1px solid var(--vp-c-divider); color: var(--vp-c-text-2); font-size: 12px; }
.target-summary strong { color: var(--vp-c-text-2); }
.download-all { display: flex; min-height: 32px; padding: 6px 10px; align-items: center; justify-content: center; gap: 8px; border: 1px solid #41b8aa; border-radius: 6px; background: #155e56; color: #f0fdfa; font-size: 12px; font-weight: 650; white-space: nowrap; cursor: pointer; }
.download-all:hover { background: #197267; }
.download-all:disabled, .tree-header button:disabled { opacity: .55; cursor: wait; }
.download-all:focus-visible, .code-action:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 2px; }
.tree-scroll { min-height: 0; flex: 1; padding: 12px 10px; overflow-x: hidden; overflow-y: auto; }

.code-panel { display: flex; min-width: 0; min-height: 0; overflow: hidden; flex-direction: column; background: #0d1117; color: #d1d7e0; }
.code-header { display: flex; min-height: 52px; flex-shrink: 0; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 18px; border-bottom: 1px solid #252b35; background: #111720; font-size: 12px; }
.file-title, .code-meta { display: flex; min-width: 0; align-items: center; gap: 9px; }
.file-title strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.code-meta { flex: 0 0 auto; color: #788391; }
.theme-picker select { width: auto; height: 30px; padding: 0 28px 0 9px; border-color: #303844; border-radius: 7px; color: #c9d1d9; background-color: #161d27; font-size: 11px; }
.code-action { display: grid; width: 32px; height: 32px; flex: 0 0 32px; place-items: center; border: 1px solid #303844; border-radius: 6px; color: #c9d1d9; background: #161d27; cursor: pointer; }
.code-action:hover { border-color: #788391; background: #252b35; }
.action-error { margin: 0; padding: 8px 18px; color: #f1b8c0; background: #1c1418; font-size: 12px; }
.file-dot { width: 8px; height: 8px; border-radius: 50%; background: #3178c6; box-shadow: 0 0 0 4px rgba(49, 120, 198, .13); }
.diagnostic-banner { padding: 8px 18px; color: #f8d477; border-bottom: 1px solid #4e4529; background: #272516; font-size: 12px; }
.diagnostic-banner summary { cursor: pointer; }
.diagnostic-banner pre { max-height: 160px; overflow: auto; margin: 8px 0 0; white-space: pre-wrap; overflow-wrap: anywhere; }
.empty-code { display: flex; min-height: 0; flex: 1; align-items: center; justify-content: center; flex-direction: column; gap: 8px; color: #778291; text-align: center; }
.empty-code strong { color: #c8d0da; font-size: 15px; }
.empty-code > span:last-child { max-width: 360px; font-size: 12px; }
.code-placeholder { display: grid; width: 52px; height: 52px; margin-bottom: 8px; place-items: center; border: 1px solid #29313c; border-radius: 14px; color: #5d6977; font: 700 15px ui-monospace, monospace; background: #111720; }
.spinner { width: 30px; height: 30px; margin-bottom: 10px; border: 3px solid #28313c; border-top-color: #2dd4bf; border-radius: 50%; animation: spin .8s linear infinite; }
.error-state { padding: 28px; }
.error-state pre { width: min(760px, 100%); max-height: 420px; overflow: auto; margin: 12px 0 0; padding: 16px; border: 1px solid #432b31; border-radius: 10px; color: #f1b8c0; background: #1c1418; font-size: 12px; line-height: 1.55; text-align: left; white-space: pre-wrap; }
.error-icon { display: grid; width: 36px; height: 36px; place-items: center; border-radius: 50%; color: #ff9daa; background: #352027; font-weight: 800; }
@keyframes spin { to { transform: rotate(360deg); } }

@media (max-width: 800px) {
  .sdk-playground { width: calc(100vw - 28px); padding-top: 28px; }
  .playground-heading { align-items: flex-start; flex-direction: column; }
  .workbench { height: auto; max-height: none; grid-template-columns: 1fr; }
  .source-panel { border-right: 0; border-bottom: 1px solid var(--vp-c-divider); }
  .tree-scroll { max-height: 230px; }
  .empty-code { min-height: 440px; }
  .code-panel { height: min(600px, calc(100dvh - 96px)); min-height: 440px; }
  .code-header { padding: 0 12px; }
  .code-meta { gap: 6px; }
}
@media (max-width: 520px) {
  .code-header { align-items: stretch; flex-direction: column; }
  .file-title { min-height: 20px; }
  .code-meta { justify-content: flex-end; flex-wrap: wrap; }
  .code-meta > span { margin-right: auto; }
}
@media (prefers-reduced-motion: reduce) {
  .spinner { animation: none; }
}
</style>
