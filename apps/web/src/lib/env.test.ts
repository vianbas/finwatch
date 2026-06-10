import { describe, expect, it } from "vitest";
import { validateEnv } from "@/lib/env";

describe("validateEnv", () => {
  it("accepts valid URLs", () => {
    const env = validateEnv({
      VITE_API_URL: "https://api.example.com",
      VITE_WS_URL: "wss://api.example.com/ws",
    });
    expect(env.apiUrl).toBe("https://api.example.com");
    expect(env.wsUrl).toBe("wss://api.example.com/ws");
  });

  it("falls back to local defaults when values are missing", () => {
    const env = validateEnv({});
    expect(env.apiUrl).toBe("http://localhost:8080");
    expect(env.wsUrl).toBe("ws://localhost:8080/ws");
  });

  it("rejects a non-http API URL", () => {
    expect(() =>
      validateEnv({ VITE_API_URL: "ftp://nope", VITE_WS_URL: "ws://x/ws" }),
    ).toThrow(/VITE_API_URL/);
  });

  it("rejects a non-ws WebSocket URL", () => {
    expect(() =>
      validateEnv({ VITE_API_URL: "http://x", VITE_WS_URL: "http://x/ws" }),
    ).toThrow(/VITE_WS_URL/);
  });
});
