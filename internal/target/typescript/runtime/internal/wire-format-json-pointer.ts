export function matchesWireJSONPointer(value: string): boolean {
  return /^(?:\/(?:[^~/]|~[01])*)*$/u.test(value);
}
