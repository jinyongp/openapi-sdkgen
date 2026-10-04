export function matchesWireRegex(value: string): boolean {
  try {
    new RegExp(value, "u");
    return true;
  } catch {
    return false;
  }
}
