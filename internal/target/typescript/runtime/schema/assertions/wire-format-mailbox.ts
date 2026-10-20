import { matchesDomain } from "./wire-format-domain.js";
import { matchesWireIPv6 } from "./wire-format-ipv6.js";
export function matchesMailbox(value: string, unicode: boolean): boolean {
  const at: number = value.lastIndexOf("@");
  if (at <= 0 || (!unicode && /[^\x00-\x7f]/u.test(value))) return false;
  const local: string = value.slice(0, at),
    domain: string = value.slice(at + 1);
  if (new TextEncoder().encode(local).length > 64) return false;
  const points: string[] = Array.from(local);
  if (local.startsWith('"')) {
    if (points.length < 2 || points.at(-1) !== '"') return false;
    for (let index: number = 1; index < points.length - 1; index++) {
      const point: number = points[index]!.codePointAt(0)!;
      if (point === 92) {
        const escaped: number = points[++index]?.codePointAt(0) ?? 0;
        if (index >= points.length - 1 || escaped < 32 || escaped > 126) return false;
      } else if (
        point < 32 ||
        point === 34 ||
        point === 127 ||
        (point >= 0xd800 && point <= 0xdfff)
      )
        return false;
    }
  } else {
    if (local.startsWith(".") || local.endsWith(".") || local.includes("..")) return false;
    for (const char of points) {
      const point: number = char.codePointAt(0)!;
      if (point >= 128) {
        if (!unicode || (point >= 0xd800 && point <= 0xdfff)) return false;
      } else if (!/[a-z0-9!#$%&'*+\-/=?^_`{|}~.]/iu.test(char)) return false;
    }
  }
  if (domain.startsWith("[") && domain.endsWith("]")) {
    const literal: string = domain.slice(1, -1);
    if (/^ipv6:/iu.test(literal)) return matchesWireIPv6(literal.slice(5));
    const octets: string[] = literal.split(".");
    return (
      octets.length === 4 &&
      octets.every((part: string): boolean => /^[0-9]{1,3}$/.test(part) && Number(part) <= 255)
    );
  }
  return matchesDomain(domain, unicode);
}
