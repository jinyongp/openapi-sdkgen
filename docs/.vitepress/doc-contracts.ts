export type SDKTarget = "typescript" | "golang";
export type DisplayLocale = "en" | "ko";
export interface DocContext { readonly sdk: SDKTarget | null; readonly locale: DisplayLocale; readonly topic: string; }
export interface DocDestination { readonly path: string; readonly fallback: boolean; }
export interface DocTopic {
  readonly id: string;
  readonly suffix: string;
  readonly targets: readonly SDKTarget[];
  readonly labels: Readonly<Record<DisplayLocale, string>>;
  readonly group: string;
}
export interface DocScope { readonly key: string; readonly sdk: SDKTarget | null; readonly locale: DisplayLocale; }
export interface LegacyEntry {
  readonly route: string;
  readonly destination: string;
  readonly sdk: SDKTarget;
  readonly locale: DisplayLocale;
  readonly topic: string;
  readonly anchors: readonly string[];
  readonly fragments: Readonly<Record<string, string>>;
}
