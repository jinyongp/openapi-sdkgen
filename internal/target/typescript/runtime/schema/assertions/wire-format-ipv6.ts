export function matchesWireIPv6(value: string): boolean {
  if (!value.includes(":")) return false;
  try {
    return new URL("http://[" + value + "]").hostname.length > 0;
  } catch {
    return false;
  }
}
