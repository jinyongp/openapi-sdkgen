export function matchesWireTime(value: string): boolean {
  const match: RegExpExecArray | null =
    /^(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/iu.exec(value);
  if (match === null) return false;
  const hour: number = Number(match[1]);
  const minute: number = Number(match[2]);
  const second: number = Number(match[3]);
  return hour <= 23 && minute <= 59 && second <= 60;
}
