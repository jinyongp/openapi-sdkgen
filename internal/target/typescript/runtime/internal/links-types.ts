/** One generated OpenAPI Link parameter assignment. */
export interface LinkParameterDefinition {
  /** Target operation input section. */
  readonly location: "path" | "query" | "headerParams" | "cookieParams";
  /** Generated TypeScript property name inside that section. */
  readonly property: string;
  /** Literal value or OpenAPI Runtime Expression. */
  readonly value: unknown;
}

/** One generated OpenAPI Link Object lowered for a source raw response. */
export interface LinkDefinition {
  readonly parameters?: readonly LinkParameterDefinition[];
  /** Literal value or OpenAPI Runtime Expression used as target request body. */
  readonly requestBody?: unknown;
}

/** Per-link invocation values. Explicit input wins over Link-derived defaults. */
export interface LinkInvocation<Input, Options, SourceInput = unknown> {
  /** Source operation input used by `$request` runtime expressions. */
  readonly sourceInput?: SourceInput;
  /** Partial target input that overrides values derived by the Link Object. */
  readonly input?: LinkInputOverride<Input>;
  /** Options applied only to the followed target operation. */
  readonly options?: Options;
}

/** Link invocation whose target operation requires per-request options. */
export type RequiredLinkInvocation<Input, Options, SourceInput = unknown> = Omit<
  LinkInvocation<Input, Options, SourceInput>,
  "options"
> & {
  readonly options: Options;
};

/** Allows one Link call to override individual parameter sections. */
export type LinkInputOverride<Input> = {
  readonly [Section in keyof Input]?: Input[Section] extends Readonly<Record<string, unknown>>
    ? Partial<Input[Section]>
    : Input[Section];
};
