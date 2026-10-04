import { createOperationSecurity } from "./http-security.js";
import { apiHeader } from "./security-api-header.js";
import { apiQuery } from "./security-api-query.js";
import { apiCookie } from "./security-api-cookie.js";
import { httpBasic } from "./security-basic.js";
import { httpBearer } from "./security-bearer.js";
import { httpCredential } from "./security-http.js";
import { oauthCredential } from "./security-oauth.js";
import { mutualTLS } from "./security-mtls.js";
import type { OperationSecurity } from "./http-types.js";
export const applyOperationSecurity: OperationSecurity = /* @__PURE__ */ createOperationSecurity({
  "apiKey.header": apiHeader,
  "apiKey.query": apiQuery,
  "apiKey.cookie": apiCookie,
  "http.basic": httpBasic,
  "http.bearer": httpBearer,
  http: httpCredential,
  oauth2: oauthCredential,
  openIdConnect: oauthCredential,
  mutualTLS: mutualTLS,
});
