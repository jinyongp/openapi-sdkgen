import { matchesWireDate } from "./wire-format-date.js";
import { matchesWireTime } from "./wire-format-time.js";
import { matchesWireDateTime } from "./wire-format-date-time.js";
import { matchesWireHostname } from "./wire-format-hostname.js";
import { matchesWireIDNHostname } from "./wire-format-idn-hostname.js";
import { matchesWireIPv4 } from "./wire-format-ipv4.js";
import { matchesWireIPv6 } from "./wire-format-ipv6.js";
import { matchesWireURI } from "./wire-format-uri.js";
import { matchesWireURITemplate } from "./wire-format-uri-template.js";
import { matchesWireDuration } from "./wire-format-duration.js";
import { matchesWireEmail } from "./wire-format-email.js";
import { matchesWireIDNEmail } from "./wire-format-idn-email.js";
import { matchesWireUUID } from "./wire-format-uuid.js";
import { matchesWireJSONPointer } from "./wire-format-json-pointer.js";
import { matchesWireRelativeJSONPointer } from "./wire-format-relative-json-pointer.js";
import { matchesWireRegex } from "./wire-format-regex.js";
/** Implements the standard JSON Schema 2020-12 format-assertion registry. Unknown formats remain application-defined annotations. */
export function matchesWireFormat(value: string, format: string): boolean {
  switch (format.toLowerCase()) {
    case "date-time":
      return matchesWireDateTime(value);
    case "date":
      return matchesWireDate(value);
    case "time":
      return matchesWireTime(value);
    case "duration":
      return matchesWireDuration(value);
    case "email":
      return matchesWireEmail(value);
    case "idn-email":
      return matchesWireIDNEmail(value);
    case "hostname":
      return matchesWireHostname(value);
    case "idn-hostname":
      return matchesWireIDNHostname(value);
    case "ipv4":
      return matchesWireIPv4(value);
    case "ipv6":
      return matchesWireIPv6(value);
    case "uri":
      return matchesWireURI(value, true, false);
    case "uri-reference":
      return matchesWireURI(value, false, false);
    case "iri":
      return matchesWireURI(value, true, true);
    case "iri-reference":
      return matchesWireURI(value, false, true);
    case "uuid":
      return matchesWireUUID(value);
    case "uri-template":
      return matchesWireURITemplate(value);
    case "json-pointer":
      return matchesWireJSONPointer(value);
    case "relative-json-pointer":
      return matchesWireRelativeJSONPointer(value);
    case "regex":
      return matchesWireRegex(value);
    default:
      return true;
  }
}
