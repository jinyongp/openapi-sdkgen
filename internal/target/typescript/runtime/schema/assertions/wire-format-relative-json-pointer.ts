export function matchesWireRelativeJSONPointer(value: string): boolean {
  return /^(?:0|[1-9][0-9]*)(?:#|(?:\/(?:[^~/]|~[01])*)*)$/u.test(value);
}
