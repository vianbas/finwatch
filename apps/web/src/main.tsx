import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "@/app/App";
import "@/index.css";

// Importing env here validates configuration at startup (it throws on a
// malformed VITE_API_URL / VITE_WS_URL) before the app renders.
import "@/lib/env";

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("Root element #root not found");
}

createRoot(rootElement).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
