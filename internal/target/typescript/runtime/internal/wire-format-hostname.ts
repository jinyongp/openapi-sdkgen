export function matchesWireHostname(value: string): boolean {
  if (value.length === 0 || value.length > 253 || /[^\x00-\x7f]/u.test(value)) return false;
  const normalized: string = value.endsWith(".") ? value.slice(0, -1) : value;
  return (
    normalized.length > 0 &&
    normalized
      .split(".")
      .every((label: string): boolean => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/iu.test(label))
  );
}
