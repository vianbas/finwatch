import { afterEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RulesPage } from "@/pages/RulesPage";
import type { Rule } from "@/lib/apiTypes";
import {
  errorResponse,
  jsonResponse,
  makeRule,
  renderPage,
  requestedUrls,
  stubApi,
} from "@/test/api";

afterEach(() => {
  vi.unstubAllGlobals();
});

const amountRule = makeRule(1, { name: "Large amount", severity: "high" });
const currencyRule = makeRule(2, {
  name: "Watched currency",
  field: "currency",
  operator: "eq",
  value: "GBP",
  severity: "low",
  enabled: false,
});

/** listHandler serves GET /rules from a mutable array. */
function listHandler(rules: Rule[]) {
  return () => jsonResponse({ items: rules });
}

describe("RulesPage", () => {
  it("shows a loading state, then a captioned table with the rule fields", async () => {
    stubApi(listHandler([amountRule, currencyRule]));
    renderPage(<RulesPage />, "/rules", { role: "operator" });
    expect(await screen.findByRole("status")).toHaveTextContent(/loading rules/i);

    const table = await screen.findByRole("table", { name: /monitoring rules/i });
    const rows = within(table).getAllByRole("row");
    expect(rows).toHaveLength(3);
    expect(within(rows[1]).getByText("Large amount")).toBeInTheDocument();
    expect(within(rows[1]).getByText("Amount ≥ 500,000 minor units")).toBeInTheDocument();
    expect(within(rows[1]).getByText("high")).toBeInTheDocument();
    expect(within(rows[1]).getByText("Enabled")).toBeInTheDocument();
    expect(within(rows[1]).getByText("2 Jan 2026, 03:04:05 UTC")).toHaveAttribute(
      "datetime",
      "2026-01-02T03:04:05Z",
    );
    expect(within(rows[2]).getByText("Currency = GBP")).toBeInTheDocument();
    expect(within(rows[2]).getByText("Disabled")).toBeInTheDocument();
    for (const name of ["Name", "Condition", "Severity", "Status", "Created"]) {
      expect(within(table).getByRole("columnheader", { name })).toBeInTheDocument();
    }
  });

  it("shows admins an Actions column with Disable/Enable buttons", async () => {
    stubApi(listHandler([amountRule, currencyRule]));
    renderPage(<RulesPage />, "/rules", { role: "admin" });

    const table = await screen.findByRole("table");
    expect(within(table).getByRole("columnheader", { name: "Actions" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Disable rule Large amount" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Enable rule Watched currency" })).toBeEnabled();
    expect(screen.queryByText("Only admins can enable or disable rules.")).not.toBeInTheDocument();
  });

  it("shows operators no toggles and a note", async () => {
    stubApi(listHandler([amountRule, currencyRule]));
    renderPage(<RulesPage />, "/rules", { role: "operator" });

    const table = await screen.findByRole("table");
    expect(within(table).queryByRole("columnheader", { name: "Actions" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /rule/i })).not.toBeInTheDocument();
    expect(screen.getByText("Only admins can enable or disable rules.")).toBeInTheDocument();
  });

  it("disables a rule with PATCH {enabled:false}, disables the button while pending and refetches", async () => {
    const rules = [{ ...amountRule }];
    let release: () => void = () => {};
    const gate = new Promise<void>((r) => (release = r));
    const api = stubApi(async (_url, init) => {
      if (init?.method === "PATCH") {
        await gate;
        rules[0].enabled = false;
        return jsonResponse(rules[0]);
      }
      return jsonResponse({ items: rules });
    });
    renderPage(<RulesPage />, "/rules", { role: "admin" });

    const button = await screen.findByRole("button", { name: "Disable rule Large amount" });
    await userEvent.click(button);
    await waitFor(() => expect(button).toBeDisabled());

    release();
    expect(await screen.findByRole("button", { name: "Enable rule Large amount" })).toBeEnabled();
    expect(screen.getByText("Disabled")).toBeInTheDocument();

    const patch = api.mock.calls.find(([, init]) => init?.method === "PATCH");
    expect(new URL(String(patch?.[0])).pathname).toBe(`/rules/${amountRule.id}`);
    expect(patch?.[1]?.body).toBe('{"enabled":false}');
    const headers = new Headers(patch?.[1]?.headers);
    expect(headers.get("Content-Type")).toBe("application/json");
    expect(headers.get("Authorization")).toBe("Bearer test-token");
    const listCalls = requestedUrls(api).filter((u) => u.pathname === "/rules");
    expect(listCalls.length).toBeGreaterThanOrEqual(2);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("enables a disabled rule with PATCH {enabled:true}", async () => {
    const api = stubApi((_url, init) =>
      init?.method === "PATCH" ? jsonResponse({ ...currencyRule, enabled: true }) : jsonResponse({ items: [currencyRule] }),
    );
    renderPage(<RulesPage />, "/rules", { role: "admin" });

    await userEvent.click(await screen.findByRole("button", { name: "Enable rule Watched currency" }));

    await waitFor(() => {
      const patch = api.mock.calls.find(([, init]) => init?.method === "PATCH");
      expect(patch?.[1]?.body).toBe('{"enabled":true}');
    });
  });

  it("explains a 403 and lets the message be dismissed", async () => {
    stubApi((_url, init) =>
      init?.method === "PATCH"
        ? errorResponse(403, "FORBIDDEN", "admin role required")
        : jsonResponse({ items: [amountRule] }),
    );
    renderPage(<RulesPage />, "/rules", { role: "admin" });

    await userEvent.click(await screen.findByRole("button", { name: "Disable rule Large amount" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Your role can't change rules.");
    expect(screen.getByRole("table")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("explains a 404 and refreshes the list", async () => {
    let gone = false;
    let listCalls = 0;
    stubApi((_url, init) => {
      if (init?.method === "PATCH") {
        gone = true;
        return errorResponse(404, "NOT_FOUND", "rule not found");
      }
      listCalls += 1;
      return jsonResponse({ items: gone ? [] : [amountRule] });
    });
    renderPage(<RulesPage />, "/rules", { role: "admin" });

    await userEvent.click(await screen.findByRole("button", { name: "Disable rule Large amount" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "This rule no longer exists. The list has been refreshed.",
    );
    await waitFor(() => expect(listCalls).toBeGreaterThanOrEqual(2));
    expect(await screen.findByText("No rules configured.")).toBeInTheDocument();
  });

  it("shows the server message for any other toggle failure", async () => {
    stubApi((_url, init) =>
      init?.method === "PATCH"
        ? errorResponse(500, "INTERNAL", "database unavailable")
        : jsonResponse({ items: [amountRule] }),
    );
    renderPage(<RulesPage />, "/rules", { role: "admin" });

    await userEvent.click(await screen.findByRole("button", { name: "Disable rule Large amount" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Could not update the rule: database unavailable",
    );
  });

  it("shows an empty state", async () => {
    stubApi(listHandler([]));
    renderPage(<RulesPage />, "/rules", { role: "admin" });
    expect(await screen.findByText("No rules configured.")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows an error state with retry", async () => {
    let fail = true;
    stubApi(() =>
      fail ? errorResponse(500, "INTERNAL", "database unavailable") : jsonResponse({ items: [amountRule] }),
    );
    renderPage(<RulesPage />, "/rules", { role: "operator" });

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Could not load rules");
    expect(alert).toHaveTextContent("database unavailable");

    fail = false;
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(await screen.findByRole("table")).toBeInTheDocument();
  });
});
