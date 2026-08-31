import { describe, it, expect } from "vitest";
import { parseSSEEvent } from "../ai/aiService";

describe("aiService parseSSEEvent", () => {
  it("parses a text event from {type, content}", () => {
    const out = parseSSEEvent({ type: "text", content: "hello" });
    expect(out).toEqual({ type: "text", content: "hello" });
  });

  it("parses a text event from a bare {content} payload", () => {
    // Backend has historically used both shapes; the parser
    // tolerates {content} or {text} or {message} on text events.
    const out = parseSSEEvent({ content: "hi" });
    expect(out?.type).toBe("text");
    if (out?.type === "text") expect(out.content).toBe("hi");
  });

  it("parses tool_call with arguments under .result", () => {
    // The Go handler ships tool args under .result for compat reasons.
    const out = parseSSEEvent({
      type: "tool_call",
      id: "call_1",
      name: "create_rectangle",
      result: { x: 10, y: 20, width: 100, height: 50 },
    });
    expect(out?.type).toBe("tool_call");
    if (out?.type === "tool_call") {
      expect(out.name).toBe("create_rectangle");
      expect(out.arguments).toEqual({ x: 10, y: 20, width: 100, height: 50 });
    }
  });

  it("falls back to .arguments / .params on tool_call", () => {
    const out = parseSSEEvent({
      type: "tool_call",
      id: "c2",
      name: "edit_text",
      arguments: { elementId: "e1", text: "hi" },
    });
    if (out?.type === "tool_call") {
      expect(out.arguments).toEqual({ elementId: "e1", text: "hi" });
    }
  });

  it("parses tool_result", () => {
    const out = parseSSEEvent({
      type: "tool_result",
      callId: "c1",
      name: "create_rectangle",
      success: true,
      result: { id: "r1" },
    });
    expect(out).toEqual({
      type: "tool_result",
      callId: "c1",
      name: "create_rectangle",
      success: true,
      result: { id: "r1" },
      error: undefined,
    });
  });

  it("parses done event", () => {
    const out = parseSSEEvent({ type: "done", summary: "ok", elementCount: 5 });
    expect(out).toEqual({ type: "done", summary: "ok", elementCount: 5 });
  });

  it("parses usage event with all token fields", () => {
    const out = parseSSEEvent({
      type: "usage",
      usage: { promptTokens: 100, completionTokens: 50, totalTokens: 150 },
    });
    expect(out).toEqual({
      type: "usage",
      usage: { promptTokens: 100, completionTokens: 50, totalTokens: 150 },
    });
  });

  it("coerces missing usage fields to 0", () => {
    const out = parseSSEEvent({ type: "usage", usage: { totalTokens: 10 } });
    expect(out?.type).toBe("usage");
    if (out?.type === "usage") {
      expect(out.usage.promptTokens).toBe(0);
      expect(out.usage.completionTokens).toBe(0);
      expect(out.usage.totalTokens).toBe(10);
    }
  });

  it("parses error event from {type:'error', content}", () => {
    const out = parseSSEEvent({ type: "error", content: "Boom" });
    expect(out).toEqual({ type: "error", message: "Boom" });
  });

  it("returns null for non-object input", () => {
    expect(parseSSEEvent(null)).toBeNull();
    expect(parseSSEEvent("string")).toBeNull();
    expect(parseSSEEvent(42)).toBeNull();
  });

  it("returns null for empty / unknown event shape", () => {
    expect(parseSSEEvent({})).toBeNull();
    expect(parseSSEEvent({ foo: "bar" })).toBeNull();
  });

  it("parses start event with requestId and maxSteps", () => {
    const out = parseSSEEvent({ type: "start", requestId: "r1", maxSteps: 20 });
    expect(out).toEqual({ type: "start", requestId: "r1", maxSteps: 20 });
  });

  it("parses start event from backend shape (content + result)", () => {
    // Backend sends {type:"start", content:"<uuid>", result:20}
    const out = parseSSEEvent({ type: "start", content: "abc-123", result: 20 });
    expect(out?.type).toBe("start");
    if (out?.type === "start") {
      expect(out.requestId).toBe("abc-123");
      expect(out.maxSteps).toBe(20);
    }
  });

  it("parses agent_iteration event", () => {
    const out = parseSSEEvent({ type: "agent_iteration", step: 3, maxSteps: 20 });
    expect(out).toEqual({ type: "agent_iteration", step: 3, maxSteps: 20 });
  });

  it("parses agent_iteration from content fallback", () => {
    const out = parseSSEEvent({ type: "agent_iteration", content: "2" });
    expect(out).toEqual({ type: "agent_iteration", step: 2, maxSteps: 20 });
  });

  it("parses agent_final event", () => {
    const out = parseSSEEvent({ type: "agent_final", reason: "max_steps" });
    expect(out).toEqual({ type: "agent_final", reason: "max_steps" });
  });

  it("normalizes unknown agent_final reason to stop", () => {
    const out = parseSSEEvent({ type: "agent_final", reason: "weird" });
    expect(out).toEqual({ type: "agent_final", reason: "stop" });
  });
});
