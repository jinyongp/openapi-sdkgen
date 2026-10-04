import type { SecuritySchemeDefinition, SecurityCredential } from "./security.js";
import type { Transport } from "./transport.js";
export interface SecurityCredentialHeader {
  readonly name: string;
  readonly value: string;
}
export interface SecurityCredentialHandler {
  validate(scheme: SecuritySchemeDefinition, credential: SecurityCredential): void;
  header?(
    scheme: SecuritySchemeDefinition,
    credential: SecurityCredential,
  ): SecurityCredentialHeader | undefined;
  apply?(
    transport: Transport | undefined,
    scheme: SecuritySchemeDefinition,
    credential: SecurityCredential,
    headers: Headers,
    url: URL,
  ): void;
}
