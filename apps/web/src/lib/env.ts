// Centralised, validated access to frontend environment configuration. No
// component should read import.meta.env directly; import `env` from here so a
// misconfiguration fails loudly and in one place.

export interface AppEnv {
  /** Base URL of the FinWatch REST API, e.g. http://localhost:8080. */
  apiUrl: string;
  /** WebSocket URL for the realtime stream, e.g. ws://localhost:8080/ws. */
  wsUrl: string;
}

type RawEnv = Record<string, string | undefined>;

// Development-friendly defaults that match the local Docker stack. They keep
// `npm run dev` and the containerised build working out of the box; an
// explicitly provided but malformed value still fails validation.
const DEFAULT_API_URL = "http://localhost:8080";
const DEFAULT_WS_URL = "ws://localhost:8080/ws";

/**
 * validateEnv resolves and validates the frontend environment. Missing values
 * fall back to local-development defaults; provided values must be well-formed
 * URLs with the correct scheme, otherwise an Error is thrown.
 */
export function validateEnv(source: RawEnv): AppEnv {
  const apiUrl = source.VITE_API_URL?.trim() || DEFAULT_API_URL;
  const wsUrl = source.VITE_WS_URL?.trim() || DEFAULT_WS_URL;

  const errors: string[] = [];
  if (!isUrlWithScheme(apiUrl, ["http:", "https:"])) {
    errors.push(`VITE_API_URL must be an http(s) URL, got "${apiUrl}"`);
  }
  if (!isUrlWithScheme(wsUrl, ["ws:", "wss:"])) {
    errors.push(`VITE_WS_URL must be a ws(s) URL, got "${wsUrl}"`);
  }
  if (errors.length > 0) {
    throw new Error(`Invalid frontend environment:\n- ${errors.join("\n- ")}`);
  }

  return { apiUrl, wsUrl };
}

function isUrlWithScheme(value: string, schemes: string[]): boolean {
  try {
    return schemes.includes(new URL(value).protocol);
  } catch {
    return false;
  }
}

export const env: AppEnv = validateEnv(
  import.meta.env as unknown as RawEnv,
);
