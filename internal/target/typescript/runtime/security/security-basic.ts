import type { SecuritySchemeDefinition, SecurityCredential } from "./security.js";
import type { SecurityCredentialHeader } from "./security-handler-types.js";
import type { SecurityCredentialHandler } from "./security-handler-types.js";
import { securityCredentialError } from "./security-diagnostics.js";
export const httpBasic: SecurityCredentialHandler = {
  validate(scheme: SecuritySchemeDefinition, credential: SecurityCredential): void {
    if (
      credential?.kind !== "http-basic" ||
      typeof credential.username !== "string" ||
      typeof credential.password !== "string"
    )
      throw securityCredentialError(scheme.name, "http-basic credential");
  },
  header(
    _scheme: SecuritySchemeDefinition,
    credential: SecurityCredential,
  ): SecurityCredentialHeader | undefined {
    const basic: import("./security.js").HTTPBasicCredential =
      credential as import("./security.js").HTTPBasicCredential;
    return {
      name: "Authorization",
      value: `Basic ${base64(`${basic.username}:${basic.password}`)}`,
    };
  },
};
function base64(value: string): string {
  const bytes: Uint8Array<ArrayBuffer> = new TextEncoder().encode(value);
  let binary: string = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}
