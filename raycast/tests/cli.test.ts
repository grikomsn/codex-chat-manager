import { describe, expect, it } from "vitest";

import { actionArgs, listArgs, parseManagerJSON, ParserError } from "../src/cli-core";
import { addToQueue, reconcileQueue, removeFromQueue, toggleQueue } from "../src/queue";

describe("cli helpers", () => {
  describe("listArgs", () => {
    it("builds list args for filters", () => {
      expect(listArgs("all")).toEqual(["sessions", "list", "--json"]);
      expect(listArgs("active")).toEqual(["sessions", "list", "--json", "--status", "active"]);
      expect(listArgs("archived")).toEqual(["sessions", "list", "--json", "--status", "archived"]);
    });
  });

  describe("actionArgs", () => {
    it("builds delete args with confirmation bypass", () => {
      expect(actionArgs("delete", ["abc"])).toEqual(["sessions", "delete", "--id", "abc", "--yes", "--json"]);
    });

    it("builds archive args without confirmation bypass", () => {
      expect(actionArgs("archive", ["abc"])).toEqual(["sessions", "archive", "--id", "abc", "--json"]);
    });

    it("builds unarchive args without confirmation bypass", () => {
      expect(actionArgs("unarchive", ["abc"])).toEqual(["sessions", "unarchive", "--id", "abc", "--json"]);
    });

    it("builds args with multiple IDs", () => {
      expect(actionArgs("delete", ["a", "b", "c"])).toEqual([
        "sessions",
        "delete",
        "--id",
        "a",
        "--id",
        "b",
        "--id",
        "c",
        "--yes",
        "--json",
      ]);
    });
  });

  describe("parseManagerJSON", () => {
    it("parses manager JSON payloads", () => {
      const groups = parseManagerJSON<{ parent: { id: string } }[]>('[{"parent":{"id":"abc"}}]');
      expect(groups).toEqual([{ parent: { id: "abc" } }]);
    });

    it("throws error for empty string", () => {
      expect(() => parseManagerJSON("")).toThrow("Empty response from codex-chat-manager");
    });

    it("throws error for whitespace-only string", () => {
      expect(() => parseManagerJSON("   ")).toThrow("Empty response from codex-chat-manager");
    });

    it("throws ParserError for invalid JSON", () => {
      expect(() => parseManagerJSON("not json")).toThrow();
    });

    it("throws error with Failed to parse message for invalid JSON", () => {
      expect(() => parseManagerJSON("not json")).toThrow("Failed to parse");
    });

    it("ParserError is exported and has snippet property", () => {
      const error = new ParserError("test message", "test snippet");
      expect(error).toBeInstanceOf(Error);
      expect(error.name).toBe("ParserError");
      expect(error.message).toBe("test message");
      expect(error.snippet).toBe("test snippet");
    });

    it("ParserError contains snippet from invalid JSON", () => {
      const invalidJSON = "not json at all";
      try {
        parseManagerJSON(invalidJSON);
        expect.fail("Should have thrown");
      } catch (error) {
        expect(error).toBeInstanceOf(ParserError);
        expect((error as ParserError).snippet).toBe(invalidJSON.slice(0, 200));
      }
    });
  });
});

describe("queue helpers", () => {
  describe("addToQueue", () => {
    it("adds id when not present", () => {
      expect(addToQueue([], "one")).toEqual(["one"]);
      expect(addToQueue(["existing"], "new")).toEqual(["existing", "new"]);
    });

    it("prevents duplicates", () => {
      expect(addToQueue(["one", "two"], "one")).toEqual(["one", "two"]);
    });

    it("handles empty array", () => {
      expect(addToQueue([], "first")).toEqual(["first"]);
    });
  });

  describe("removeFromQueue", () => {
    it("removes id when present", () => {
      expect(removeFromQueue(["one", "two"], "one")).toEqual(["two"]);
    });

    it("returns same array when id not present", () => {
      expect(removeFromQueue(["one", "two"], "three")).toEqual(["one", "two"]);
    });

    it("handles empty array", () => {
      expect(removeFromQueue([], "one")).toEqual([]);
    });
  });

  describe("toggleQueue", () => {
    it("adds id when not present", () => {
      expect(toggleQueue([], "one")).toEqual(["one"]);
      expect(toggleQueue(["existing"], "new")).toEqual(["existing", "new"]);
    });

    it("removes id when present", () => {
      expect(toggleQueue(["one", "two"], "one")).toEqual(["two"]);
      expect(toggleQueue(["one"], "one")).toEqual([]);
    });
  });

  describe("reconcileQueue", () => {
    it("filters to known IDs", () => {
      expect(reconcileQueue(["one", "two", "three"], ["one", "three"])).toEqual(["one", "three"]);
    });

    it("returns empty array when no matches", () => {
      expect(reconcileQueue(["a", "b"], ["c", "d"])).toEqual([]);
    });

    it("handles empty ids array", () => {
      expect(reconcileQueue([], ["a", "b"])).toEqual([]);
    });

    it("handles empty knownIDs array", () => {
      expect(reconcileQueue(["a", "b"], [])).toEqual([]);
    });
  });

  it("adds, removes, and reconciles ids", () => {
    const added = addToQueue([], "one");
    expect(added).toEqual(["one"]);
    expect(removeFromQueue(["one", "two"], "one")).toEqual(["two"]);
    expect(reconcileQueue(["one", "two"], ["two"])).toEqual(["two"]);
  });
});
