import type { WireSchema } from "./wire-types.js";
export function validateMultipleOf(value: number, schema: WireSchema): void {
  if (schema.multipleOf !== undefined && !isMultipleOf(value, schema.multipleOf))
    throw new TypeError(`must be a multiple of ${schema.multipleOf}`);
}
function isMultipleOf(value: number, divisor: number): boolean {
  if (!Number.isFinite(value) || !Number.isFinite(divisor) || divisor <= 0) return false;
  const [numerator, numeratorScale]: readonly [bigint, number] = decimalInteger(value);
  const [denominator, denominatorScale]: readonly [bigint, number] = decimalInteger(divisor);
  const scale: number = numeratorScale - denominatorScale;
  // Number's finite decimal representation bounds this exponent to 632.
  return scale >= 0
    ? (numerator * 10n ** BigInt(scale)) % denominator === 0n
    : numerator % (denominator * 10n ** BigInt(-scale)) === 0n;
}

function decimalInteger(value: number): readonly [bigint, number] {
  const [mantissa = "0", exponent = "0"]: string[] = String(value).split("e");
  const point: number = mantissa.indexOf(".");
  const fractional: number = point < 0 ? 0 : mantissa.length - point - 1;
  return [BigInt(mantissa.replace(".", "")), Number(exponent) - fractional];
}
