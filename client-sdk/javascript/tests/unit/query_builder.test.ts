import { describe, it, expect, vi } from "vitest";
import { createClient } from "../../src/core/client";
import { QueryBuilder } from "../../src/extensions/query-builder";

describe("Fluent QueryBuilder Unit Tests", () => {
  const createMockClient = (mockResponseBody: any, status = 200) => {
    let capturedUrl = "";
    let capturedMethod = "";
    let capturedBody: any = null;

    const mockFetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      capturedUrl = String(input);
      capturedMethod = init?.method || "GET";
      capturedBody = init?.body ? JSON.parse(String(init.body)) : null;

      return new Response(JSON.stringify(mockResponseBody), {
        status,
        headers: { "Content-Type": "application/json" },
      });
    });

    const client = createClient({
      endpoint: "http://localhost:5297",
      fetch: mockFetch,
    });

    return {
      client,
      getCaptured: () => ({ url: capturedUrl, method: capturedMethod, body: capturedBody }),
    };
  };

  it("QB-01: Projections - select columns", async () => {
    const { client, getCaptured } = createMockClient([{ id: 1, title: "Item" }]);
    const qb = new QueryBuilder(client, "items");

    const response = await qb.select(["id", "title"]);
    expect(response).toEqual([{ id: 1, title: "Item" }]);
    expect(getCaptured().url).toContain("select=id%2Ctitle");

    await new QueryBuilder(client, "items").select("*");
    expect(getCaptured().url).toContain("select=*");
  });

  it("QB-02 & QB-03: Filtering: equality & inequality", async () => {
    const { client, getCaptured } = createMockClient([]);
    await new QueryBuilder(client, "users").eq("status", "active").neq("role", "guest");
    expect(getCaptured().url).toContain("status=eq.active");
    expect(getCaptured().url).toContain("role=neq.guest");
  });

  it("QB-04: Filtering: comparisons (gt, gte, lt, lte)", async () => {
    const { client, getCaptured } = createMockClient([]);
    await new QueryBuilder(client, "products")
      .gt("price", 10)
      .gte("stock", 0)
      .lt("discount", 50)
      .lte("weight", 100);

    expect(getCaptured().url).toContain("price=gt.10");
    expect(getCaptured().url).toContain("stock=gte.0");
    expect(getCaptured().url).toContain("discount=lt.50");
    expect(getCaptured().url).toContain("weight=lte.100");
  });

  it("QB-05: Filtering: pattern matching (like, ilike)", async () => {
    const { client, getCaptured } = createMockClient([]);
    await new QueryBuilder(client, "articles").like("title", "%postgres%").ilike("author", "john%");

    expect(getCaptured().url).toContain("title=like.%25postgres%25");
    expect(getCaptured().url).toContain("author=ilike.john%25");
  });

  it("QB-06 & QB-07: Array membership (in) and null predicate (is)", async () => {
    const { client, getCaptured } = createMockClient([]);
    await new QueryBuilder(client, "orders").in("id", [1, 2, 3]).is("deleted_at", null);

    expect(getCaptured().url).toContain("id=in.(1%2C2%2C3)");
    expect(getCaptured().url).toContain("deleted_at=is.null");

    await new QueryBuilder(client, "orders").is("archived", "not.null");
    expect(getCaptured().url).toContain("archived=is.not.null");
  });

  it("QB-08: Modifiers: Order by", async () => {
    const { client, getCaptured } = createMockClient([]);
    await new QueryBuilder(client, "logs").order("created_at", {
      ascending: false,
      nullsFirst: true,
    });

    expect(getCaptured().url).toContain("order=created_at.desc.nullsfirst");

    await new QueryBuilder(client, "logs").order("id", { ascending: true });
    expect(getCaptured().url).toContain("order=id.asc.nullslast");
  });

  it("uses default join columns and default order placement", async () => {
    const { client, getCaptured } = createMockClient([]);
    await new QueryBuilder(client, "items").join("profiles").order("id");
    const url = new URL(getCaptured().url);
    expect(url.searchParams.get("select")).toBe("*,profiles(*)");
    expect(url.searchParams.get("order")).toBe("id.asc");
  });

  it("QB-09: Modifiers: Pagination (limit, offset, range)", async () => {
    const { client, getCaptured } = createMockClient([]);
    await new QueryBuilder(client, "items").limit(10).offset(20);
    expect(getCaptured().url).toContain("limit=10");
    expect(getCaptured().url).toContain("offset=20");

    await new QueryBuilder(client, "items").range(10, 19);
    expect(getCaptured().url).toContain("limit=10");
    expect(getCaptured().url).toContain("offset=10");
  });

  it("QB-10: Terminal: single() returns exact row or throws on 0 / 2+ rows", async () => {
    const { client: c1 } = createMockClient([{ id: 1, name: "Target" }]);
    const item = await new QueryBuilder(c1, "items").eq("id", 1).single();
    expect(item).toEqual({ id: 1, name: "Target" });

    const { client: cEmpty } = createMockClient([]);
    await expect(new QueryBuilder(cEmpty, "items").single()).rejects.toThrow(
      "Expected single record, found 0",
    );

    const { client: cMany } = createMockClient([{ id: 1 }, { id: 2 }]);
    await expect(new QueryBuilder(cMany, "items").single()).rejects.toThrow(
      "Expected single record, found 2",
    );
  });

  it("QB-11: Terminal: maybeSingle() returns row, null on 0, or throws on 2+", async () => {
    const { client: c1 } = createMockClient([{ id: 1 }]);
    expect(await new QueryBuilder(c1, "items").maybeSingle()).toEqual({ id: 1 });

    const { client: cEmpty } = createMockClient([]);
    expect(await new QueryBuilder(cEmpty, "items").maybeSingle()).toBeNull();

    const { client: cMany } = createMockClient([{ id: 1 }, { id: 2 }]);
    await expect(new QueryBuilder(cMany, "items").maybeSingle()).rejects.toThrow(
      "Expected at most one record, found 2",
    );
  });

  it("QB-12: Mutation: insert executes POST", async () => {
    const { client, getCaptured } = createMockClient({ id: 10, title: "New Todo" });
    const response = await new QueryBuilder(client, "todos").insert({ title: "New Todo" });

    expect(getCaptured().method).toBe("POST");
    expect(getCaptured().body).toEqual({ title: "New Todo" });
    expect(response).toEqual({ id: 10, title: "New Todo" });
  });

  it("QB-13: Mutation: update executes PUT or PATCH with primaryKey", async () => {
    const { client, getCaptured } = createMockClient({ id: 5, status: "done" });
    const response = await new QueryBuilder(client, "todos").eq("id", 5).update({ status: "done" });

    expect(getCaptured().method).toBe("PATCH");
    expect(getCaptured().url).toContain("/v1/data/public/todos/5");
    expect(getCaptured().body).toEqual({ status: "done" });
    expect(response).toEqual({ id: 5, status: "done" });
  });

  it("QB-14: Mutation: delete executes DELETE", async () => {
    const { client, getCaptured } = createMockClient(null);
    await new QueryBuilder(client, "todos").eq("id", 5).delete();

    expect(getCaptured().method).toBe("DELETE");
    expect(getCaptured().url).toContain("/v1/data/public/todos/5");
  });

  it("QB-15: Mutation: upsert executes POST with onConflict header/query", async () => {
    const { client, getCaptured } = createMockClient({ id: 1 });
    await new QueryBuilder(client, "todos").upsert(
      { id: 1, title: "Upsert" },
      { onConflict: "id" },
    );

    expect(getCaptured().method).toBe("POST");
    expect(getCaptured().url).toContain("on_conflict=id");
  });

  it("QB-16: Schema Routing: parses schema and table correctly", async () => {
    const { client, getCaptured } = createMockClient([]);
    await new QueryBuilder(client, "analytics.pageviews").select("*");
    expect(getCaptured().url).toContain("/v1/data/analytics/pageviews");
  });

  it("QB-17: Error handling in execute throws LayrAPIError", async () => {
    const { client } = createMockClient({ detail: "Table not found" }, 404);
    await expect(new QueryBuilder(client, "missing_table").select("*")).rejects.toThrow(
      "Table not found",
    );
  });

  it("QB-18: Type-safe column array selection narrows return type", async () => {
    interface TodoItem {
      id: number;
      title: string;
      completed: boolean;
    }
    const { client } = createMockClient([{ id: 1, title: "Test" }]);
    const qb = new QueryBuilder<TodoItem>(client, "todos");
    const response = await qb.select(["id", "title"]);
    expect(response).toEqual([{ id: 1, title: "Test" }]);
  });

  it("QB-19: Nested path filtering and JSON arrows in eq", async () => {
    interface ComplexDoc {
      id: number;
      meta: {
        score: number;
        author: { name: string };
      };
    }
    const { client, getCaptured } = createMockClient([]);
    const qb = new QueryBuilder<ComplexDoc>(client, "docs");
    await qb.eq("meta.score", 99).eq("meta.author.name", "Alice").eq("meta->>tag", "important");

    expect(getCaptured().url).toContain("meta.score=eq.99");
    expect(getCaptured().url).toContain("meta.author.name=eq.Alice");
    expect(getCaptured().url).toContain("meta-%3E%3Etag=eq.important");
  });

  it("QB-20: Fluent join attaches relation to select and augments result", async () => {
    interface PostRecord {
      id: number;
      title: string;
      user_id: number;
    }
    interface UserRecord {
      id: number;
      name: string;
      email: string;
    }
    const { client, getCaptured } = createMockClient([
      {
        id: 1,
        title: "Post 1",
        user_id: 10,
        author: { id: 10, name: "Alice", email: "a@example.com" },
      },
    ]);
    const qb = new QueryBuilder<PostRecord>(client, "posts");
    const response = await qb.select(["id", "title"]).join<UserRecord>("users", {
      columns: ["id", "name", "email"],
      as: "author",
    });

    expect(getCaptured().url).toContain("users(id%2Cname%2Cemail)");
    expect(response[0]?.author?.name).toBe("Alice");
  });

  it("QB-21: catch and finally on QueryBuilder, QueryBuilderSingle, and QueryBuilderMaybeSingle", async () => {
    const { client } = createMockClient({ detail: "fail" }, 500);
    const qb = new QueryBuilder(client, "items");

    let caught = false;
    await qb.catch(() => {
      caught = true;
    });
    expect(caught).toBe(true);

    const singleQb = new QueryBuilder(client, "items").single();
    let singleCaught = false;
    await singleQb.catch(() => {
      singleCaught = true;
    });
    expect(singleCaught).toBe(true);

    const maybeSingleQb = new QueryBuilder(client, "items").maybeSingle();
    let maybeCaught = false;
    await maybeSingleQb.catch(() => {
      maybeCaught = true;
    });
    expect(maybeCaught).toBe(true);

    const { client: clientOk } = createMockClient([{ id: 1 }]);
    let qbFinalized = false;
    await new QueryBuilder(clientOk, "items").finally(() => {
      qbFinalized = true;
    });
    expect(qbFinalized).toBe(true);

    let singleFinalized = false;
    await new QueryBuilder(clientOk, "items").single().finally(() => {
      singleFinalized = true;
    });
    expect(singleFinalized).toBe(true);

    let maybeFinalized = false;
    await new QueryBuilder(clientOk, "items").maybeSingle().finally(() => {
      maybeFinalized = true;
    });
    expect(maybeFinalized).toBe(true);
  });

  it("QB-22: maybeSingle returns null when 0 records, throws when > 1 records", async () => {
    const { client: clientEmpty } = createMockClient([]);
    const resultEmpty = await new QueryBuilder(clientEmpty, "items").maybeSingle();
    expect(resultEmpty).toBeNull();

    const { client: clientMultiple } = createMockClient([{ id: 1 }, { id: 2 }]);
    await expect(new QueryBuilder(clientMultiple, "items").maybeSingle()).rejects.toThrow(
      "Expected at most one record, found 2",
    );
  });

  it("QB-23: upsert and update without id validation", async () => {
    const { client } = createMockClient({ id: 1, name: "Upserted" });
    const qb = new QueryBuilder(client, "items");
    const upsertRes = await qb.upsert({ id: 1, name: "Upserted" });
    expect(upsertRes).toEqual({ id: 1, name: "Upserted" });

    const batchRes = await qb.insert([{ name: "Batch 1" }, { name: "Batch 2" }]);
    expect(batchRes).toBeDefined();

    const qbNoId = new QueryBuilder(client, "items");
    await expect(qbNoId.update({ name: "test" })).rejects.toThrow("update requires a record ID");
    await expect(qbNoId.delete()).rejects.toThrow("delete requires a record ID");
  });

  it("QB-24: buildQueryURL generates proper URL string", () => {
    const { client } = createMockClient([]);
    const qbDefault = new QueryBuilder(client, "items");
    expect(qbDefault.buildQueryURL()).toBe("http://localhost:5297/v1/data/public/items?select=*");

    const qbFull = new QueryBuilder(client, "app.items")
      .select("id,name")
      .eq("status", "active")
      .order("id", { ascending: true })
      .limit(10)
      .offset(20);

    expect(qbFull.buildQueryURL()).toBe(
      "http://localhost:5297/v1/data/app/items?select=id%2Cname&status=eq.active&order=id.asc.nullslast&limit=10&offset=20",
    );

    // Test with no query parts
    (qbDefault as any).selectCols = "";
    expect(qbDefault.buildQueryURL()).toBe("http://localhost:5297/v1/data/public/items");
  });

  it("QB-25: QueryBuilder branch edge cases (empty select, string join columns, malformed filter, non-array batch insert)", async () => {
    const { client, getCaptured } = createMockClient({ id: 100, name: "single-wrapped" });

    // 1. select with empty string falls back to '*'
    const qbEmptySelect = new QueryBuilder(client, "items");
    qbEmptySelect.select("");
    expect((qbEmptySelect as any).selectCols).toBe("*");

    // 2. join with string columns and inner join type
    const qbJoin = new QueryBuilder(client, "users");
    qbJoin.join("posts", { columns: "id,title", type: "inner" });
    expect((qbJoin as any).selectCols).toContain("posts!inner(id,title)");

    // 3. exec with empty selectCols and a filter param without '='
    const qbFilterEdge = new QueryBuilder(client, "items");
    (qbFilterEdge as any).selectCols = "";
    (qbFilterEdge as any).filterParams.push("malformed_filter_without_equals");
    await qbFilterEdge;
    expect(getCaptured().url).not.toContain("select=");
    expect(getCaptured().url).not.toContain("malformed_filter");

    // 4. insert array of records where backend mock responds with a single object (non-array)
    const qbBatch = new QueryBuilder(client, "items");
    const batchResult = await qbBatch.insert([{ name: "item1" }, { name: "item2" }]);
    expect(Array.isArray(batchResult)).toBe(true);
    expect(batchResult).toEqual([{ id: 100, name: "single-wrapped" }]);

    // 5. insert array of records where backend mock responds with an array
    const { client: clientArrayResp } = createMockClient([{ id: 101, name: "array-item" }]);
    const qbBatchArray = new QueryBuilder(clientArrayResp, "items");
    const batchArrayResult = await qbBatchArray.insert([{ name: "item1" }]);
    expect(batchArrayResult).toEqual([{ id: 101, name: "array-item" }]);
  });
});
