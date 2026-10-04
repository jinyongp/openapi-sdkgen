export function matchesWireIDNEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+$/u.test(value);
}
