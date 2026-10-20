import { idnaBidiClass, idnaJoiningType, idnaVirama } from "./wire-format-idna-context.js";
/** UTS 46 mapping with IDNA2008 label, ContextO/ContextJ and Bidi checks. */
export function matchesDomain(value: string, unicode: boolean): boolean {
  if (
    value.length === 0 ||
    /[\s\x00-\x20\x7f:/?#@%\\\[\]]/u.test(value) ||
    (!unicode && /[^\x00-\x7f]/u.test(value))
  )
    return false;
  try {
    // The suffix keeps numeric hostnames out of URL IPv4 normalization.
    const mapped: string = new URL("http://" + value + ".invalid").hostname.slice(0, -8);
    if (mapped.length === 0 || mapped.length > 253) return false;
    const labels: string[] = mapped.split(".");
    const decoded: string[] = [];
    for (const label of labels) {
      if (label.length > 63 || !/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/iu.test(label)) return false;
      const text: string = label.startsWith("xn--") ? decodePunycode(label.slice(4)) : label;
      if (
        label.startsWith("xn--") &&
        (/^[\x00-\x7f]*$/u.test(text) ||
          new URL("http://" + text + ".invalid").hostname !== label + ".invalid")
      )
        return false;
      if (!validLabel(text)) return false;
      decoded.push(text);
    }
    const bidi: boolean = decoded.some((label: string): boolean =>
      Array.from(label).some((char: string): boolean =>
        ["R", "AL", "AN"].includes(idnaBidiClass(char)),
      ),
    );
    return !bidi || decoded.every(validBidi);
  } catch {
    return false;
  }
}
function validLabel(label: string): boolean {
  const points: string[] = Array.from(label);
  if (
    points[0] === "-" ||
    points.at(-1) === "-" ||
    /^\p{M}/u.test(label) ||
    (points[2] === "-" && points[3] === "-") ||
    /[\u0640\u07fa\u302e\u302f\u3031-\u3035\u303b\u1100-\u11ff]/u.test(label) ||
    /[^\p{L}\p{M}\p{Nd}\-\u00b7\u0375\u05f3\u05f4\u30fb\u200c\u200d\u0f0b\u3007\u06fd\u06fe]/u.test(
      label,
    )
  )
    return false;
  if (/[\u0660-\u0669]/u.test(label) && /[\u06f0-\u06f9]/u.test(label)) return false;
  for (let index: number = 0; index < points.length; index++) {
    const char: string = points[index]!;
    const previous: string = points[index - 1] ?? "";
    const next: string = points[index + 1] ?? "";
    if (char === "\u00b7" && (previous !== "l" || next !== "l")) return false;
    if (char === "\u0375" && !/\p{Script=Greek}/u.test(next)) return false;
    if ((char === "\u05f3" || char === "\u05f4") && !/\p{Script=Hebrew}/u.test(previous))
      return false;
    if (char === "\u30fb" && !/[\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Han}]/u.test(label))
      return false;
    if (char !== "\u200c" && char !== "\u200d") continue;
    if (previous !== "" && idnaVirama(previous)) continue;
    if (char === "\u200d") return false;
    let left: number = index - 1,
      right: number = index + 1;
    while (left >= 0 && idnaJoiningType(points[left]!) === "T") left--;
    while (right < points.length && idnaJoiningType(points[right]!) === "T") right++;
    if (
      left < 0 ||
      right >= points.length ||
      !["L", "D"].includes(idnaJoiningType(points[left]!)) ||
      !["R", "D"].includes(idnaJoiningType(points[right]!))
    )
      return false;
  }
  return true;
}
function validBidi(label: string): boolean {
  const points: string[] = Array.from(label);
  const first: string = idnaBidiClass(points[0]!);
  const rtl: boolean = first === "R" || first === "AL";
  if (!rtl && first !== "L") return false;
  let last: string = "",
    en: boolean = false,
    an: boolean = false;
  for (const point of points) {
    const kind: string = idnaBidiClass(point);
    if ((["R", "AL", "AN"].includes(kind) && !rtl) || (kind === "L" && rtl)) return false;
    if (!["R", "AL", "AN", "L", "EN", "ES", "CS", "ET", "ON", "BN", "NSM"].includes(kind))
      return false;
    if (kind !== "NSM") last = kind;
    en ||= kind === "EN";
    an ||= kind === "AN";
  }
  return rtl ? !(en && an) && ["R", "AL", "EN", "AN"].includes(last) : ["L", "EN"].includes(last);
}
// RFC 3492 decoder, bounded to Unicode scalar values and safe integer arithmetic.
function decodePunycode(input: string): string {
  const output: number[] = [];
  const delimiter: number = input.lastIndexOf("-");
  let offset: number = 0,
    n: number = 128,
    i: number = 0,
    bias: number = 72;
  if (delimiter >= 0) {
    for (const char of input.slice(0, delimiter)) {
      if (char.codePointAt(0)! >= 128) throw new TypeError("Punycode basic character");
      output.push(char.charCodeAt(0));
    }
    offset = delimiter + 1;
  }
  while (offset < input.length) {
    const old: number = i;
    let weight: number = 1;
    for (let k: number = 36; ; k += 36) {
      if (offset >= input.length) throw new TypeError("Punycode truncated");
      const point: number = input.charCodeAt(offset++);
      const digit: number =
        point >= 48 && point <= 57
          ? point - 22
          : point >= 65 && point <= 90
            ? point - 65
            : point >= 97 && point <= 122
              ? point - 97
              : 36;
      if (digit >= 36 || !Number.isSafeInteger(i + digit * weight))
        throw new TypeError("Punycode digit");
      i += digit * weight;
      const threshold: number = k <= bias + 1 ? 1 : k >= bias + 26 ? 26 : k - bias;
      if (digit < threshold) break;
      weight *= 36 - threshold;
      if (!Number.isSafeInteger(weight)) throw new TypeError("Punycode overflow");
    }
    const count: number = output.length + 1;
    let delta: number = old === 0 ? Math.floor((i - old) / 700) : Math.floor((i - old) / 2);
    delta += Math.floor(delta / count);
    let k: number = 0;
    while (delta > 455) {
      delta = Math.floor(delta / 35);
      k += 36;
    }
    bias = k + Math.floor((36 * delta) / (delta + 38));
    n += Math.floor(i / count);
    i %= count;
    if (n > 0x10ffff || (n >= 0xd800 && n <= 0xdfff)) throw new TypeError("Punycode scalar");
    output.splice(i, 0, n);
    i++;
  }
  return String.fromCodePoint(...output);
}
