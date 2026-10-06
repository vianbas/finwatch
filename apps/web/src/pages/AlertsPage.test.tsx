import { afterEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AlertsPage } from "@/pages/AlertsPage";
import type { Alert } from "@/lib/apiTypes";
import {
  errorResponse,
  jsonResponse,
  makeAlert,
  renderPage,
  requestedUrls,
  stubApi,
} from "@/test/api";

afterEach(() => {
  vi.unstubAllGlobals();
});

const open = makeAlert(1, { status: "open", severity: "high" });
const acked = makeAlert(2, { status: "acknowledged", severity: "medium" });
const resolved = makeAlert(3, { status: "resolved", severity: "low" });

/** listHandler serves GET /alerts from a mutable array, honouring the status filter. */
function listHandler(alerts: Alert[]) {
  return (url: URL) => {
    const status = url.searchParams.get("status");
    return jsonResponse({
      items: alerts.filter((a) => !status || a.status === status),
      nextCursor: null,
    });
  };
}

describe("AlertsPage", () => {
  it("shows a loading state, then a captioned table with the alert fields", async () => {
    stubApi(listHandler([open]));
    renderPage(<AlertsPage />, "/alerts");
    expect(screen.getByRole("status")).toHaveTextContent(/loading alerts/i);

    const table = await screen.findByRole("table", { name: /all alerts, most recent first/i });
    const row = within(table).getAllByRole("row")[1];
    expect(within(row).getByText("high")).toBeInTheDocument();
    expect(within(row).getByText("open")).toBeInTheDocument();
    expect(within(row).getByText(open.transactionId)).toBeInTheDocument();
    expect(within(row).getByText("2 Jan 2026, 03:04:05 UTC")).toHaveAttribute(
      "datetime",
      "2026-01-02T03:04:05Z",
    );
    expect(within(row).getByText("2 Jan 2026, 03:05:06 UTC")).toBeInTheDocument();
    for (const name of ["Severity", "Status", "Created", "Updated", "Transaction", "Actions"]) {
      expect(within(table).getByRole("columnheader", { name })).toBeInTheDocument();
    }
  });

  it("offers Acknowledge only for open alerts and Resolve only for acknowledged ones", async () => {
    stubApi(listHandler([open, acked, resolved]));
    renderPage(<AlertsPage />, "/alerts");
    await screen.findByRole("table");

    expect(screen.getByRole("button", { name: `Acknowledge alert ${open.id.slice(0, 8)}` })).toBeEnabled();
    expect(screen.getByRole("button", { name: `Resolve alert ${acked.id.slice(0, 8)}` })).toBeEnabled();
    expect(screen.getAllByRole("button", { name: /^acknowledge alert/i })).toHaveLength(1);
    expect(screen.getAllByRole("button", { name: /^resolve alert/i })).toHaveLength(1);
    expect(screen.getAllByText("No actions available")).toHaveLength(1);
  });

  it("acknowledges an alert, disables the button while pending and refreshes the list", async () => {
    const alerts = [{ ...open }];
    let release: () => void = () => {};
    const gate = new Promise<void>((r) => (release = r));
    const api = stubApi(async (url, init) => {
      if (init?.method === "POST") {
        await gate;
        alerts[0].status = "acknowledged";
        return jsonResponse(alerts[0]);
      }
      return listHandler(alerts)(url);
    });
    renderPage(<AlertsPage />, "/alerts");

    const button = await screen.findByRole("button", { name: /^acknowledge alert/i });
    await userEvent.click(button);
    await waitFor(() => expect(button).toBeDisabled());

    release();
    expect(
      await screen.findByRole("button", { name: `Resolve alert ${open.id.slice(0, 8)}` }),
    ).toBeEnabled();
    const post = api.mock.calls.find(([, init]) => init?.method === "POST");
    expect(new URL(String(post?.[0])).pathname).toBe(`/alerts/${open.id}/acknowledge`);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows a non-fatal inline message on 409 INVALID_TRANSITION and refreshes the list", async () => {
    // The server already moved the alert on; the page's list is stale until the refetch.
    let serverStatus: Alert["status"] = "open";
    let listCalls = 0;
    stubApi((_url, init) => {
      if (init?.method === "POST") {
        serverStatus = "resolved";
        return errorResponse(409, "INVALID_TRANSITION", "alert is not in a state that allows this transition");
      }
      listCalls += 1;
      return jsonResponse({ items: [{ ...open, status: listCalls === 1 ? "open" : serverStatus }], nextCursor: null });
    });
    renderPage(<AlertsPage />, "/alerts");

    await userEvent.click(await screen.findByRole("button", { name: /^acknowledge alert/i }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /can no longer be acknowledged.*list has been refreshed/i,
    );
    await waitFor(() => expect(listCalls).toBeGreaterThanOrEqual(2));
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: /^acknowledge alert/i })).not.toBeInTheDocument(),
    );
    // The table is still there: the failure is not fatal.
    expect(screen.getByRole("table")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("stores the status filter in the URL and requests it", async () => {
    const api = stubApi(listHandler([open, acked, resolved]));
    renderPage(<AlertsPage />, "/alerts");
    await screen.findByRole("table");
    expect(requestedUrls(api)[0].searchParams.get("status")).toBeNull();

    await userEvent.selectOptions(screen.getByLabelText("Status"), "acknowledged");

    expect(screen.getByTestId("location")).toHaveTextContent("/alerts?status=acknowledged");
    await waitFor(() => {
      const last = requestedUrls(api).at(-1);
      expect(last?.searchParams.get("status")).toBe("acknowledged");
    });
    expect(await screen.findByRole("table", { name: /status acknowledged/i })).toBeInTheDocument();
    expect(screen.getAllByRole("row")).toHaveLength(2);

    await userEvent.selectOptions(screen.getByLabelText("Status"), "");
    expect(screen.getByTestId("location")).toHaveTextContent(/^\/alerts$/);
  });

  it("reads the initial filter from the URL and ignores unknown values", async () => {
    const api = stubApi(listHandler([open]));
    renderPage(<AlertsPage />, "/alerts?status=open");
    await screen.findByRole("table");
    expect(screen.getByLabelText("Status")).toHaveValue("open");
    expect(requestedUrls(api)[0].searchParams.get("status")).toBe("open");
  });

  it("falls back to all alerts for an invalid status in the URL", async () => {
    const api = stubApi(listHandler([open]));
    renderPage(<AlertsPage />, "/alerts?status=bogus");
    await screen.findByRole("table");
    expect(screen.getByLabelText("Status")).toHaveValue("");
    expect(requestedUrls(api)[0].searchParams.get("status")).toBeNull();
  });

  it("loads more with the cursor and hides the button on the last page", async () => {
    const api = stubApi((url) =>
      url.searchParams.get("cursor") === "cur-2"
        ? jsonResponse({ items: [makeAlert(2)], nextCursor: null })
        : jsonResponse({ items: [makeAlert(1)], nextCursor: "cur-2" }),
    );
    renderPage(<AlertsPage />, "/alerts");
    await screen.findByRole("table");
    expect(screen.getAllByRole("row")).toHaveLength(2);

    await userEvent.click(screen.getByRole("button", { name: "Load more" }));
    await waitFor(() => expect(screen.getAllByRole("row")).toHaveLength(3));
    expect(requestedUrls(api).at(-1)?.searchParams.get("cursor")).toBe("cur-2");
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
  });

  it("shows an empty state", async () => {
    stubApi(listHandler([]));
    renderPage(<AlertsPage />, "/alerts?status=resolved");
    expect(await screen.findByText("No resolved alerts.")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows an error state with retry", async () => {
    let fail = true;
    stubApi((url) =>
      fail ? errorResponse(500, "INTERNAL", "database unavailable") : listHandler([open])(url),
    );
    renderPage(<AlertsPage />, "/alerts");

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Could not load alerts");
    expect(alert).toHaveTextContent("database unavailable");

    fail = false;
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("table")).toBeInTheDocument();
  });
});
