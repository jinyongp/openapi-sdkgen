const codeThemeKey = "openapi-sdkgen.playground.code-theme.v1";
const expandedPathsKey = "openapi-sdkgen.playground.expanded-paths.v1";

export interface PreferenceStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

export interface PlaygroundPreferences<Theme extends string> {
  readonly codeTheme?: Theme;
  readonly expandedPaths: readonly string[];
}

export function readPlaygroundPreferences<Theme extends string>(
  storage: PreferenceStorage | undefined,
  supportedThemes: readonly Theme[],
  sdk?: string,
): PlaygroundPreferences<Theme> {
  if (!storage) return { expandedPaths: [] };
  try {
    const storedTheme = storage.getItem(codeThemeKey);
    const codeTheme = supportedThemes.find((theme) => theme === storedTheme);
    const storedPaths: unknown = JSON.parse(storage.getItem(sdk ? `${expandedPathsKey}.${sdk}` : expandedPathsKey) ?? "[]");
    const expandedPaths = Array.isArray(storedPaths)
      ? [...new Set(storedPaths.filter((path): path is string => typeof path === "string" && path.length > 0))]
      : [];
    return codeTheme === undefined ? { expandedPaths } : { codeTheme, expandedPaths };
  } catch {
    return { expandedPaths: [] };
  }
}

export interface WritablePlaygroundPreferences { readonly codeTheme: string; readonly expandedPaths: ReadonlySet<string>; }

export function writePlaygroundPreferences(
  storage: PreferenceStorage | undefined,
  preferences: WritablePlaygroundPreferences,
  sdk?: string,
): void {
  if (!storage) return;
  try {
    storage.setItem(codeThemeKey, preferences.codeTheme);
    storage.setItem(sdk ? `${expandedPathsKey}.${sdk}` : expandedPathsKey, JSON.stringify([...preferences.expandedPaths].sort()));
  } catch {
    // Storage may be disabled or full. Preferences remain usable for this page session.
  }
}

export function readSelectedPath(storage: PreferenceStorage | undefined, sdk: string, paths: readonly string[]): string {
  try { const selected: string | null | undefined = storage?.getItem(`openapi-sdkgen.playground.selected.${sdk}.v1`); return selected && paths.includes(selected) ? selected : paths[0] ?? ""; } catch { return paths[0] ?? ""; }
}
export function writeSelectedPath(storage: PreferenceStorage | undefined, sdk: string, path: string): void {
  try { storage?.setItem(`openapi-sdkgen.playground.selected.${sdk}.v1`, path); } catch { /* Storage is optional. */ }
}
