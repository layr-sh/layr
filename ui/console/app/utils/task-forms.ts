import type { FormError } from "@nuxt/ui";

export interface TaskTargetForm {
  advanced: boolean;
  raw: string;
  url: string;
  headers: string;
  body: string;
  query: string;
  params: string;
  extra: Record<string, unknown>;
}
export function createTaskTarget(): TaskTargetForm {
  return {
    advanced: false,
    raw: "{}",
    url: "",
    headers: "",
    body: "",
    query: "",
    params: "",
    extra: {},
  };
}
export function loadTaskTarget(taskTargetForm: TaskTargetForm, payload: object) {
  const value = payload as Record<string, unknown>;
  Object.assign(taskTargetForm, {
    raw: JSON.stringify(value, null, 2),
    url: typeof value.url === "string" ? value.url : "",
    headers: Object.entries((value.headers || {}) as Record<string, string>)
      .map(([name, content]) => `${name}: ${content}`)
      .join("\n"),
    body: value.body === undefined ? "" : JSON.stringify(value.body, null, 2),
    query: typeof value.query === "string" ? value.query : "",
    params: value.params === undefined ? "" : JSON.stringify(value.params, null, 2),
    extra: Object.fromEntries(
      Object.entries(value).filter(
        ([key]) => !["url", "headers", "body", "query", "params"].includes(key),
      ),
    ),
  });
}
export function resolveTaskTarget(taskTargetForm: TaskTargetForm, targetType: string) {
  const errors: (FormError & { name: string })[] = [];
  const parse = (value: string, name: string): unknown => {
    if (!value.trim()) return undefined;
    try {
      return JSON.parse(value);
    } catch {
      errors.push({ name, message: "Enter valid JSON." });
      return undefined;
    }
  };
  let payload: Record<string, unknown> = { ...taskTargetForm.extra };
  if (taskTargetForm.advanced) {
    const raw = parse(taskTargetForm.raw, "target.raw");
    if (!raw || typeof raw !== "object" || Array.isArray(raw))
      errors.push({
        name: "target.raw",
        message: "Enter a JSON object, not an array or primitive.",
      });
    else payload = raw as Record<string, unknown>;
  } else if (targetType === "http") {
    payload.url = taskTargetForm.url.trim();
    if (taskTargetForm.headers.trim()) {
      const headers: Record<string, string> = Object.create(null);
      for (const line of taskTargetForm.headers.split(/\r?\n/).filter((line) => line.trim())) {
        const separator = line.indexOf(":");
        const name = line.slice(0, separator).trim();
        if (
          separator < 1 ||
          !/^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/.test(name) ||
          Object.keys(headers).some((key) => key.toLowerCase() === name.toLowerCase())
        ) {
          errors.push({
            name: "target.headers",
            message: "Use one unique Header-Name: value per line.",
          });
        } else headers[name] = line.slice(separator + 1).trim();
      }
      payload.headers = headers;
    }
    if (taskTargetForm.body.trim()) payload.body = parse(taskTargetForm.body, "target.body");
  } else {
    payload.query = taskTargetForm.query.trim();
    if (taskTargetForm.params.trim())
      payload.params = parse(taskTargetForm.params, "target.params");
  }
  const field = (name: string) => (taskTargetForm.advanced ? "target.raw" : `target.${name}`);
  if (targetType === "http") {
    try {
      const url = new URL(String(payload.url));
      if (!["http:", "https:"].includes(url.protocol)) throw new Error("protocol");
    } catch {
      errors.push({ name: field("url"), message: "Enter a complete HTTP or HTTPS URL." });
    }
    if (
      payload.headers !== undefined &&
      (!payload.headers ||
        typeof payload.headers !== "object" ||
        Array.isArray(payload.headers) ||
        Object.entries(payload.headers).some(
          ([name, value]) =>
            !/^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/.test(name) ||
            typeof value !== "string" ||
            /[\r\n]/.test(value),
        ))
    ) {
      errors.push({
        name: field("headers"),
        message: "Headers must be an object of valid header names and single-line string values.",
      });
    }
    if ("query" in payload)
      errors.push({ name: field("url"), message: "An HTTP target must not contain a SQL query." });
  } else {
    if (typeof payload.query !== "string" || !payload.query.trim())
      errors.push({ name: field("query"), message: "Enter a SQL query." });
    if (payload.params !== undefined && !Array.isArray(payload.params))
      errors.push({
        name: field("params"),
        message: 'Parameters must be a JSON array, such as [42, "value"].',
      });
  }
  return { payload, errors };
}

export const taskSchedules = [
  { label: "Every minute", value: "* * * * *", summary: "Runs every minute" },
  {
    label: "Every 15 minutes",
    value: "*/15 * * * *",
    summary: "Runs at minutes 0, 15, 30, and 45 of every hour",
  },
  { label: "Hourly", value: "0 * * * *", summary: "Runs every hour at minute 0" },
  { label: "Daily at midnight", value: "0 0 * * *", summary: "Runs every day at midnight" },
  { label: "Weekly on Sunday", value: "0 0 * * 0", summary: "Runs every Sunday at midnight" },
  {
    label: "Monthly on the 1st",
    value: "0 0 1 * *",
    summary: "Runs on the first day of each month at midnight",
  },
  { label: "Custom cron expression", value: "custom", summary: "Custom cron schedule" },
];
export const taskTimezones = ["UTC", ...Intl.supportedValuesOf("timeZone")];
export function validateTaskCron(expression: string): string | undefined {
  const aliases = ["@hourly", "@daily", "@midnight", "@weekly", "@monthly", "@yearly", "@annually"];
  if (aliases.includes(expression.trim().toLowerCase())) return undefined;
  const fields = expression.trim().split(/\s+/);
  if (fields.length !== 5)
    return "Use five cron fields: minute, hour, day of month, month, day of week.";
  const bounds = [
    [0, 59],
    [0, 23],
    [1, 31],
    [1, 12],
    [0, 7],
  ] as const;
  for (const [index, field] of fields.entries()) {
    const [min, max] = bounds[index]!;
    for (const part of field.split(",")) {
      const match = /^(\*|\d+(?:-\d+)?)(?:\/([1-9]\d*))?$/.exec(part);
      if (!match)
        return "Use numeric values, *, lists, ranges, or positive steps in each cron field.";
      if (match[1] !== "*") {
        const [start, end = start] = match[1]!.split("-").map(Number);
        if (start! < min || end! > max || start! > end!)
          return `Cron field ${index + 1} must be within ${min}–${max}, with ranges in ascending order.`;
      }
    }
  }
  return undefined;
}
