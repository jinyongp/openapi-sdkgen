export function matchesWireDate(value: string): boolean {
  const match: RegExpExecArray | null = /^(\d{4})-(\d{2})-(\d{2})$/u.exec(value);
  if (match === null) return false;
  const year: number = Number(match[1]);
  const month: number = Number(match[2]);
  const day: number = Number(match[3]);
  const date: Date = new Date(Date.UTC(year, month - 1, day));
  return (
    date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day
  );
}
