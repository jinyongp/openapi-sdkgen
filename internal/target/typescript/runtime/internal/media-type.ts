/** Shared media-type predicates. */
export { isJSONMediaType, isXMLMediaType } from "./runtime-support.js";
/** Classifies built-in binary media without retaining any body implementation. */
export function isBinaryMediaType(contentType: string): boolean {
  return (
    contentType === "application/octet-stream" ||
    contentType.startsWith("image/") ||
    contentType.startsWith("audio/") ||
    contentType.startsWith("video/")
  );
}
