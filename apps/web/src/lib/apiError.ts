/**
 * ApiError is a failed API response parsed from the canonical error envelope
 * `{"error":{"code","message"}}` (contracts/openapi.yaml, Error schema).
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

/** toApiError reads a non-2xx response; a malformed body still yields an ApiError. */
export async function toApiError(res: Response): Promise<ApiError> {
  let code = "UNKNOWN";
  let message = `Request failed (${res.status})`;
  try {
    const body: unknown = await res.json();
    const err = (body as { error?: { code?: unknown; message?: unknown } } | null)?.error;
    if (typeof err?.code === "string") code = err.code;
    if (typeof err?.message === "string") message = err.message;
  } catch {
    // Body was not JSON; keep the generic message.
  }
  return new ApiError(res.status, code, message);
}

/** requestJson runs a request and returns its JSON body, throwing ApiError on non-2xx. */
export async function requestJson<T>(
  apiFetch: (path: string, init?: RequestInit) => Promise<Response>,
  path: string,
  init?: RequestInit,
): Promise<T> {
  const res = await apiFetch(path, init);
  if (!res.ok) throw await toApiError(res);
  return (await res.json()) as T;
}

/** errorMessage is a display-safe message for any thrown value. */
export function errorMessage(error: unknown): string {
  return error instanceof ApiError
    ? error.message
    : "Something went wrong. Check your connection and try again.";
}
