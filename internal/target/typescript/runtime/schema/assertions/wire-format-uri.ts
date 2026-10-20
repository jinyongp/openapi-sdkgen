import { matchesWireIPv6 } from "./wire-format-ipv6.js";
/** RFC 3986/3987 syntax validation; URL parsing would normalize invalid input. */
export function matchesWireURI(value: string, absolute: boolean, allowUnicode: boolean): boolean {
  const match: RegExpExecArray | null =
    /^(?:([a-zA-Z][a-zA-Z0-9+.-]*):)?(?:\/\/([^/?#]*))?([^?#]*)(?:\?([^#]*))?(?:#([\s\S]*))?$(?![\s\S])/u.exec(
      value,
    );
  if (match === null || (absolute && match[1] === undefined)) return false;
  const path: string = match[3]!;
  if (match[1] === undefined && match[2] === undefined && path.split("/", 1)[0]!.includes(":"))
    return false;
  if (
    !uriPart(path, "/:@", allowUnicode, false) ||
    !uriPart(match[4] ?? "", "/:@?", allowUnicode, true) ||
    !uriPart(match[5] ?? "", "/:@?", allowUnicode, false)
  )
    return false;
  if (match[2] === undefined) return true;
  let host: string = match[2];
  const at: number = host.indexOf("@");
  if (at >= 0) {
    if (!uriPart(host.slice(0, at), ":", allowUnicode, false)) return false;
    host = host.slice(at + 1);
  }
  if (host.startsWith("[")) {
    const end: number = host.indexOf("]");
    if (end < 0 || !/^(?::[0-9]*)?$(?![\s\S])/u.test(host.slice(end + 1))) return false;
    const literal: string = host.slice(1, end);
    return (
      matchesWireIPv6(literal) ||
      /^[vV][0-9a-fA-F]+\.[a-zA-Z0-9._~!$&'()*+,;=:-]+$(?![\s\S])/u.test(literal)
    );
  }
  const colon: number = host.lastIndexOf(":");
  if (colon >= 0) {
    if (/[^0-9]/u.test(host.slice(colon + 1))) return false;
    host = host.slice(0, colon);
  }
  return uriPart(host, "", allowUnicode, false);
}
function uriPart(value: string, extra: string, unicode: boolean, query: boolean): boolean {
  for (let index: number = 0; index < value.length; index++) {
    const point: number = value.codePointAt(index)!;
    if (point > 0xffff) index++;
    if (point === 37) {
      if (!/^[0-9a-f]{2}$/iu.test(value.slice(index + 1, index + 3))) return false;
      index += 2;
      continue;
    }
    if (point < 128) {
      const char: string = String.fromCodePoint(point);
      if (!/[a-z0-9._~!$&'()*+,;=-]/iu.test(char) && !extra.includes(char)) return false;
      continue;
    }
    const ucs: boolean =
      (point >= 0xa0 && point <= 0xd7ff) ||
      (point >= 0xf900 && point <= 0xfdcf) ||
      (point >= 0xfdf0 && point <= 0xffef) ||
      (point >= 0x10000 && point <= 0xefffd && (point & 0xffff) <= 0xfffd);
    const privateUse: boolean =
      (point >= 0xe000 && point <= 0xf8ff) ||
      (point >= 0xf0000 && point <= 0x10fffd && (point & 0xffff) <= 0xfffd);
    if (!unicode || (!ucs && !(query && privateUse))) return false;
  }
  return true;
}
