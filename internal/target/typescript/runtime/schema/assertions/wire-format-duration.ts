export function matchesWireDuration(value: string): boolean {
  return /^P(?!$)(?:\d+Y)?(?:\d+M)?(?:\d+D)?(?:T(?=\d)(?:\d+H)?(?:\d+M)?(?:\d+(?:\.\d+)?S)?)?$/i.test(
    value,
  );
}
