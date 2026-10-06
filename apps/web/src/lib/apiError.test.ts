import { describe, expect, it } from "vitest";
import { ApiError, errorMessage, toApiError } from "@/lib/apiError";

describe("toApiError", () => {
  it("parses the canonical error envelope", async () => {
    const res = new Response(
      JSON.stringify({ error: { code: "INVALID_TRANSITION", message: "not allowed" } }),
      { status: 409 },
    );
    const err = await toApiError(res);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(409);
    expect(err.code).toBe("INVALID_TRANSITION");
    expect(err.message).toBe("not allowed");
  });

  it("falls back to a generic error for a non-JSON body", async () => {
    const err = await toApiError(new Response("<html>bad gateway</html>", { status: 502 }));
    expect(err.status).toBe(502);
    expect(err.code).toBe("UNKNOWN");
    expect(err.message).toBe("Request failed (502)");
  });

  it("ignores an envelope with the wrong shape", async () => {
    const err = await toApiError(new Response(JSON.stringify({ error: "nope" }), { status: 500 }));
    expect(err.code).toBe("UNKNOWN");
  });
});

describe("errorMessage", () => {
  it("shows an ApiError message and hides unexpected error details", () => {
    expect(errorMessage(new ApiError(400, "INVALID_QUERY", "limit must be between 1 and 200"))).toBe(
      "limit must be between 1 and 200",
    );
    expect(errorMessage(new TypeError("Failed to fetch"))).toMatch(/something went wrong/i);
  });
});
