import { afterEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { DashboardPage } from "@/pages/DashboardPage";
import type { Alert } from "@/lib/apiTypes";
import {
  errorResponse,
  jsonResponse,
  makeAlert,
  makeTransaction,
  renderPage,
  requestedUrls,
  stubApi,
} from "@/test/api";

afterEach(() => {
  vi.unstubAllGlobals();
});

interface Fixture {
  open?: Alert[];
  openNext?: string | null;
  acknowledged?: Alert[];
  acknowledgedNext?: string | null;
  transactions?: ReturnType<typeof makeTransaction>[];
}

function serve(f: Fixture) {
  return stubApi((url) => {
    if (url.pathname === "/transactions") {
      return jsonResponse({ items: f.transactions ?? [], nextCursor: null });
    }
    if (url.searchParams.get("status") === "open") {
      return jsonResponse({ items: f.open ?? [], nextCursor: f.openNext ?? null });
    }
    return jsonResponse({ items: f.acknowledged ?? [], nextCursor: f.acknowledgedNext ?? null });
  });
}

const tile = (name: string) => screen.getByRole("region", { name });

describe("DashboardPage", () => {
  it("shows alert counts, a severity breakdown with all four severities, and transactions", async () => {
    const api = serve({
      open: [
        makeAlert(1, { severity: "critical" }),
        makeAlert(2, { severity: "low" }),
        makeAlert(3, { severity: "low" }),
      ],
      acknowledged: [makeAlert(4, { status: "acknowledged" })],
      transactions: [
        makeTransaction(1, { amountMinor: 1234, currency: "USD", direction: "credit" }),
        makeTransaction(2, { amountMinor: 5000, currency: "JPY" }),
      ],
    });
    renderPage(<DashboardPage />, "/dashboard");

    expect(await within(tile("Open alerts")).findByText("3")).toBeInTheDocument();
    expect(await within(tile("Acknowledged alerts")).findByText("1")).toBeInTheDocument();

    // The chart's accessible equivalent is a table with every severity, zero-filled.
    const table = await screen.findByRole("table", { name: "Open alerts by severity" });
    const counts = within(table)
      .getAllByRole("row")
      .slice(1)
      .map((r) => [
        within(r).getByRole("rowheader").textContent,
        within(r).getByRole("cell").textContent,
      ]);
    expect(counts).toEqual([
      ["Low", "2"],
      ["Medium", "0"],
      ["High", "0"],
      ["Critical", "1"],
    ]);

    const txTable = await screen.findByRole("table", { name: "Most recent transactions" });
    expect(within(txTable).getByText("$12.34")).toBeInTheDocument();
    expect(within(txTable).getByText("¥5,000")).toBeInTheDocument();
    expect(within(txTable).getByText("ACC-1001")).toBeInTheDocument();

    const urls = requestedUrls(api);
    const alertUrls = urls.filter((u) => u.pathname === "/alerts");
    expect(alertUrls.map((u) => u.searchParams.get("status")).sort()).toEqual(["acknowledged", "open"]);
    expect(alertUrls.every((u) => u.searchParams.get("limit") === "200")).toBe(true);
    expect(urls.find((u) => u.pathname === "/transactions")?.searchParams.get("limit")).toBe("10");
  });

  it('shows "200+" when more alerts exist beyond the first page', async () => {
    serve({
      open: Array.from({ length: 200 }, (_, i) => makeAlert(i + 1)),
      openNext: "more",
      acknowledged: [makeAlert(500, { status: "acknowledged" })],
    });
    renderPage(<DashboardPage />, "/dashboard");

    expect(await within(tile("Open alerts")).findByText("200+")).toBeInTheDocument();
    expect(within(tile("Acknowledged alerts")).getByText("1")).toBeInTheDocument();
    expect(await screen.findByText(/200 most recent open alerts/i)).toBeInTheDocument();
  });

  it("shows loading states, then empty states with zeroed counts", async () => {
    serve({});
    renderPage(<DashboardPage />, "/dashboard");
    expect(screen.getByText("Loading transactions…")).toBeInTheDocument();

    expect(await screen.findByText("No transactions yet.")).toBeInTheDocument();
    expect(await within(tile("Open alerts")).findByText("0")).toBeInTheDocument();
    expect(within(tile("Acknowledged alerts")).getByText("0")).toBeInTheDocument();
    expect(screen.getByText(/no open alerts/i)).toBeInTheDocument();
    // All four severities are still listed when there is nothing open.
    const table = screen.getByRole("table", { name: "Open alerts by severity" });
    expect(within(table).getAllByRole("row")).toHaveLength(5);
  });

  it("shows error states per section without breaking the page", async () => {
    stubApi((url) =>
      url.pathname === "/transactions"
        ? errorResponse(500, "INTERNAL", "transactions store unavailable")
        : errorResponse(503, "NOT_READY", "alerts store unavailable"),
    );
    renderPage(<DashboardPage />, "/dashboard");

    expect(await screen.findByText("Could not load transactions")).toBeInTheDocument();
    expect(screen.getByText("transactions store unavailable")).toBeInTheDocument();
    expect(await screen.findByText("Could not load open alerts")).toBeInTheDocument();
    expect(screen.getByText("alerts store unavailable")).toBeInTheDocument();
    expect(within(tile("Open alerts")).getByText("Unavailable")).toBeInTheDocument();
    expect(within(tile("Acknowledged alerts")).getByText("Unavailable")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Retry" })).toHaveLength(2);
    expect(screen.getByRole("heading", { name: "Dashboard" })).toBeInTheDocument();
  });
});
