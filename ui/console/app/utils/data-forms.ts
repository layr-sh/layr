import type { FormError } from "@nuxt/ui";

export const dataTypes = [
  "text",
  "uuid",
  "boolean",
  "integer",
  "bigint",
  "numeric",
  "float8",
  "jsonb",
  "date",
  "timestamp with time zone",
  "time",
  "json",
  "smallint",
  "real",
  "varchar",
  "bytea",
];
export function validDataIdentifier(value: string) {
  return /^[A-Za-z_][A-Za-z0-9_]*$/.test(value) && value.length <= 63;
}
export function identifierErrors(state: Record<string, unknown>, names: string[]): FormError[] {
  return names
    .filter((name) => !validDataIdentifier(String(state[name] ?? "")))
    .map((name) => ({
      name,
      message:
        "Use a name of up to 63 letters, numbers, or underscores, starting with a letter or underscore.",
    }));
}
export function displayDataValue(value: unknown): string {
  if (value === null) return "NULL";
  if (value === undefined) return "—";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}
export function parseDataRecord(value: string): Record<string, unknown> {
  const parsed: unknown = JSON.parse(value);
  if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed))
    throw new Error("Enter a JSON object with column names as keys.");
  if (!Object.keys(parsed).length) throw new Error("Include at least one column value.");
  return parsed as Record<string, unknown>;
}
export function quoteDataIdentifier(value: string) {
  return `"${value.replaceAll('"', '""')}"`;
}
export function estimatedDataRows(value: number) {
  return value < 0 ? "Not estimated" : `~${value.toLocaleString()}`;
}
