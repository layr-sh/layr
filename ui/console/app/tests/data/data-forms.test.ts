import { describe, expect, it } from "vitest";
import {
  dataTypes,
  validDataIdentifier,
  identifierErrors,
  displayDataValue,
  parseDataRecord,
  quoteDataIdentifier,
  estimatedDataRows,
} from "../../utils/data-forms";
describe("Data forms", () => {
  it("validates identifiers without allowing SQL fragments", () => {
    expect(dataTypes).toContain("jsonb");
    for (const value of ["table_name", "_name", "A123"])
      expect(validDataIdentifier(value)).toBe(true);
    for (const value of ["", "1name", "a.b", "a;drop", "a".repeat(64)])
      expect(validDataIdentifier(value)).toBe(false);
    expect(identifierErrors({ name: "ok" }, ["name"])).toEqual([]);
    expect(identifierErrors({}, ["name"])).toHaveLength(1);
    expect(identifierErrors({ name: "" }, ["name"])[0]?.name).toBe("name");
  });
  it("renders values, quotes names, and labels estimates truthfully", () => {
    expect([null, undefined, { a: 1 }, [1], false, 1, "text"].map(displayDataValue)).toEqual([
      "NULL",
      "—",
      '{"a":1}',
      "[1]",
      "false",
      "1",
      "text",
    ]);
    expect(quoteDataIdentifier('a"b')).toBe('"a""b"');
    expect(estimatedDataRows(-1)).toBe("Not estimated");
    expect(estimatedDataRows(12)).toBe("~12");
  });
  it("accepts JSON objects and rejects ambiguous row payloads", () => {
    expect(parseDataRecord('{"title":"Hello", "value":null}')).toEqual({
      title: "Hello",
      value: null,
    });
    for (const value of ["null", "[]", "1", '"text"', "{}", "invalid"])
      expect(() => parseDataRecord(value)).toThrow();
  });
});
