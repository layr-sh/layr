import { describe, expect, it } from "vitest";
import {
  createTaskTarget,
  loadTaskTarget,
  resolveTaskTarget,
  taskSchedules,
  taskTimezones,
  validateTaskCron,
} from "../../utils/task-forms";

describe("Task form validation and serialization", () => {
  it("round-trips HTTP and SQL targets without losing optional or unknown settings", () => {
    const taskTarget = createTaskTarget();
    const http = {
      url: "https://example.com/task",
      headers: { Authorization: "Bearer token", "X-Trace": "a:b" },
      body: { run: true },
      custom: "keep",
    };
    loadTaskTarget(taskTarget, http);
    expect(resolveTaskTarget(taskTarget, "http")).toEqual({ payload: http, errors: [] });
    loadTaskTarget(taskTarget, { query: "SELECT $1", params: [42], custom: "keep" });
    expect(resolveTaskTarget(taskTarget, "sql")).toEqual({
      payload: { query: "SELECT $1", params: [42], custom: "keep" },
      errors: [],
    });
    loadTaskTarget(taskTarget, { body: null, url: 42, query: 42 });
    expect(taskTarget.body).toBe("null");
    expect(taskTarget.url).toBe("");
    expect(taskTarget.query).toBe("");
    expect(taskTarget.params).toBe("");
  });

  it("validates structured HTTP URLs, headers, and JSON bodies", () => {
    const taskTarget = createTaskTarget();
    expect(resolveTaskTarget(taskTarget, "http").errors).toContainEqual(
      expect.objectContaining({ name: "target.url" }),
    );
    taskTarget.url = "ftp://example.com/task";
    expect(resolveTaskTarget(taskTarget, "http").errors).not.toHaveLength(0);
    taskTarget.url = " https://example.com/task ";
    taskTarget.headers = "X-Trace: a:b\n\nX-Empty: ";
    taskTarget.body = "false";
    expect(resolveTaskTarget(taskTarget, "http")).toEqual({
      payload: {
        url: "https://example.com/task",
        headers: { "X-Trace": "a:b", "X-Empty": "" },
        body: false,
      },
      errors: [],
    });
    for (const headers of [
      "no colon",
      ": value",
      "Bad Name: value",
      "X-A: 1\nx-a: 2",
      "X-A: bad\rvalue",
    ]) {
      taskTarget.headers = headers;
      expect(resolveTaskTarget(taskTarget, "http").errors).not.toHaveLength(0);
    }
    taskTarget.headers = "";
    taskTarget.body = "{";
    expect(resolveTaskTarget(taskTarget, "http").errors).toContainEqual({
      name: "target.body",
      message: "Enter valid JSON.",
    });
  });

  it("validates structured SQL and parameter arrays", () => {
    const taskTarget = createTaskTarget();
    expect(resolveTaskTarget(taskTarget, "sql").errors).toContainEqual(
      expect.objectContaining({ name: "target.query" }),
    );
    taskTarget.query = " SELECT $1 ";
    taskTarget.params = "[42]";
    expect(resolveTaskTarget(taskTarget, "sql")).toEqual({
      payload: { query: "SELECT $1", params: [42] },
      errors: [],
    });
    taskTarget.params = "{}";
    expect(resolveTaskTarget(taskTarget, "sql").errors).toContainEqual(
      expect.objectContaining({ name: "target.params" }),
    );
    taskTarget.params = "";
    expect(resolveTaskTarget(taskTarget, "sql").errors).toEqual([]);
  });

  it.each(["", "{", "null", "[]", '"text"', "42"])(
    "rejects non-object advanced payload %s",
    (raw) => {
      const taskTarget = { ...createTaskTarget(), advanced: true, raw };
      expect(resolveTaskTarget(taskTarget, "http").errors).toContainEqual(
        expect.objectContaining({ name: "target.raw" }),
      );
    },
  );

  it.each([null, "text", [], { "Bad Name": "value" }, { "X-A": 42 }, { "X-A": "bad\nvalue" }])(
    "rejects invalid advanced headers %j",
    (headers) => {
      const taskTarget = {
        ...createTaskTarget(),
        advanced: true,
        raw: JSON.stringify({ url: "https://example.com", headers }),
      };
      expect(resolveTaskTarget(taskTarget, "http").errors).toContainEqual(
        expect.objectContaining({ name: "target.raw" }),
      );
    },
  );

  it("validates advanced targets using their selected driver", () => {
    const taskTarget = {
      ...createTaskTarget(),
      advanced: true,
      raw: '{"url":"https://example.com","headers":{"X-A":"ok"}}',
    };
    expect(resolveTaskTarget(taskTarget, "http").errors).toEqual([]);
    taskTarget.raw = '{"url":"https://example.com","query":"SELECT 1"}';
    expect(resolveTaskTarget(taskTarget, "http").errors).not.toHaveLength(0);
    taskTarget.raw = '{"query":42,"params":null}';
    expect(resolveTaskTarget(taskTarget, "sql").errors).toHaveLength(2);
    taskTarget.raw = '{"query":"   "}';
    expect(resolveTaskTarget(taskTarget, "sql").errors).toHaveLength(1);
    taskTarget.raw = '{"query":"SELECT 1","params":[]}';
    expect(resolveTaskTarget(taskTarget, "sql").errors).toEqual([]);
  });

  it("only generates schedules supported by the backend", () => {
    expect(taskTimezones).toContain("UTC");
    expect(taskTimezones).toContain("Asia/Jakarta");
    for (const schedule of taskSchedules.filter((schedule) => schedule.value !== "custom"))
      expect(validateTaskCron(schedule.value)).toBeUndefined();
    for (const value of [
      "@hourly",
      "@DAILY",
      "@midnight",
      "@weekly",
      "@monthly",
      "@yearly",
      "@annually",
      "0 9 * * 1-5",
      "0,15,30 1-23/2 1 1 7",
      "5/2 0 1 1 0",
    ])
      expect(validateTaskCron(value)).toBeUndefined();
  });

  it.each([
    "",
    "@unsupported",
    "* * * * * *",
    "*/0 * * * *",
    "0 9 * * MON",
    "0,,1 0 * * *",
    "60 * * * *",
    "0 24 * * *",
    "0 0 0 * *",
    "0 0 * 13 *",
    "0 0 * * 8",
    "5-1 * * * *",
  ])("rejects unsupported cron %s", (value) => {
    expect(validateTaskCron(value)).toBeTruthy();
  });
});
