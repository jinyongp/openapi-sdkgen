export function matchesWireIPv4(value: string): boolean {
  const segments: string[] = value.split(".");
  return (
    segments.length === 4 &&
    segments.every(
      (segment: string): boolean =>
        /^(?:0|[1-9][0-9]{0,2})$/u.test(segment) && Number(segment) <= 255,
    )
  );
}
