import { describe, expect, it } from "vitest";

import { actionArgs, listArgs, parseManagerJSON } from "../src/cli-core";
import { addToQueue, reconcileQueue, removeFromQueue } from "../src/queue";

describe("cli helpers", () => {
  it("builds list args for filters", () => {
    expect(listArgs("all")).toEqual(["sessions", "list", "--json"]);
    expect(listArgs("active")).toEqual(["sessions", "list", "--json", "--status", "active"]);
  });

  it("builds delete args with confirmation bypass", () => {
    expect(actionArgs("delete", ["abc"])).toEqual(["sessions", "delete", "--id", "abc", "--yes", "--json"]);
  });

  it("parses manager JSON payloads", () => {
    const groups = parseManagerJSON<{ parent: { id: string } }[]>('[{"parent":{"id":"abc"}}]');
    expect(groups).toEqual([{ parent: { id: "abc" } }]);
  });
});

describe("queue helpers", () => {
  it("adds, removes, and reconciles ids", () => {
    const added = addToQueue([], "one");
    expect(added).toEqual(["one"]);
    expect(removeFromQueue(["one", "two"], "one")).toEqual(["two"]);
    expect(reconcileQueue(["one", "two"], ["two"])).toEqual(["two"]);
  });
});
