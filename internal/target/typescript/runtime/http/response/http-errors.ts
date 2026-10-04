import { APIError, isRecord } from "../../shared/runtime-support.js";

type ErrorEnvelope<Data> = Data extends { readonly error: infer Envelope }
  ? Envelope extends object
    ? Envelope
    : Data
  : Data;
type DeclaredErrorCode<Status extends number, Envelope> = Envelope extends {
  readonly code: infer Code;
}
  ? Code extends string
    ? Code
    : `HTTP_${Status}`
  : [Envelope] extends [void]
    ? `HTTP_${Status}`
    : string;
type DeclaredErrorDetails<Envelope> = Envelope extends object
  ? "details" extends keyof Envelope
    ? Envelope["details"]
    : "fields" extends keyof Envelope
      ? Envelope["fields"]
      : unknown
  : unknown;

/** Declared HTTP failure, distributed over exact statuses and body variants. */
export type HTTPErrorFor<
  Status extends number,
  Data,
  ContentType extends string | undefined,
> = Status extends number
  ? Data extends unknown
    ? APIError<
        DeclaredErrorCode<Status, ErrorEnvelope<Data>>,
        DeclaredErrorDetails<ErrorEnvelope<Data>>,
        Status,
        Data
      > & {
        readonly status: Status;
        readonly data: Data;
        readonly contentType: ContentType;
      }
    : never
  : never;

declare const httpErrorTypeBrand: unique symbol;
/** Type-only contract carried by each exact, raw, resource and streaming method. */
export interface HTTPErrorIdentity<Failure> {
  readonly [httpErrorTypeBrand]?: Failure;
}
/** Declared HTTP failure for one generated callable, without transport or unknown fallback. */
export type OperationHTTPError<Method> =
  Method extends HTTPErrorIdentity<infer Failure> ? Failure : never;

interface HTTPErrorEvidence {
  readonly route: string;
  readonly status: number | undefined;
  readonly code: string;
  readonly data: unknown;
  readonly response: Response | undefined;
  readonly contentType: string | undefined;
  readonly actualContentType: string | null;
  readonly validate: (data: unknown) => void;
}

const methods: WeakMap<object, string> = /* @__PURE__ */ new WeakMap();
const failures: WeakMap<APIError, HTTPErrorEvidence> = /* @__PURE__ */ new WeakMap();

/** Registers runtime provenance independently of the erased public type brands. */
export function registerHTTPErrorMethod<Method extends object>(
  method: Method,
  route: string,
): Method {
  methods.set(method, route);
  return method;
}

/** Keeps resource wrappers on their source operation's identity. */
export function inheritHTTPErrorMethod<Method extends object>(
  method: Method,
  source: object,
): Method {
  const route: string | undefined = methods.get(source);
  if (route !== undefined) methods.set(method, route);
  return method;
}

/** Only real decoded server failures are registered; constructors cannot forge provenance. */
export function registerHTTPError(
  error: APIError,
  route: string,
  validate: (data: unknown) => void,
): APIError {
  failures.set(error, {
    route,
    status: error.status,
    code: error.code,
    data: error.data,
    response: error.response,
    contentType: error.contentType,
    actualContentType: error.response?.headers.get("content-type") ?? null,
    validate,
  });
  return error;
}

/** Revalidates a declared failure from the same SDK and target operation. */
export function isOperationHTTPError<Method>(
  error: unknown,
  method: Method,
): error is OperationHTTPError<Method> {
  if (!(error instanceof APIError) || typeof method !== "function") return false;
  const evidence: HTTPErrorEvidence | undefined = failures.get(error);
  if (evidence === undefined || methods.get(method) !== evidence.route) return false;
  if (
    error.status !== evidence.status ||
    error.code !== evidence.code ||
    error.data !== evidence.data ||
    error.response !== evidence.response ||
    error.contentType !== evidence.contentType ||
    (error.response?.headers.get("content-type") ?? null) !== evidence.actualContentType
  )
    return false;
  const envelope: unknown =
    isRecord(error.data) && isRecord(error.data["error"]) ? error.data["error"] : error.data;
  const record: Record<string, unknown> = isRecord(envelope) ? envelope : {};
  const code: string = typeof record["code"] === "string" ? record["code"] : `HTTP_${error.status}`;
  if (
    error.code !== code ||
    error.details !== (record["details"] ?? record["fields"]) ||
    error.fields !== record["fields"]
  )
    return false;
  try {
    evidence.validate(error.data);
    return true;
  } catch {
    return false;
  }
}
