export function matchesWireURI(value: string, absolute: boolean, allowUnicode: boolean): boolean {
  if (/[\u0000-\u001f\u007f\s]/u.test(value) || (!allowUnicode && /[^\x00-\x7f]/u.test(value)))
    return false;
  try {
    const parsed: URL = new URL(value, "https://format.invalid/");
    return !absolute || (/^[a-z][a-z0-9+.-]*:/iu.test(value) && parsed.protocol !== "");
  } catch {
    return false;
  }
}
