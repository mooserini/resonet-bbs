const { test, expect } = require("@playwright/test");
const fs = require("fs");
const path = require("path");

// Run both download handlers from the shipped script, without DOM workarounds.
const source = fs.readFileSync(path.resolve(__dirname, "../../../cmd/wolfbbs-web/modern_ui_script.go"), "utf8");
const script = source.slice(source.indexOf('<script'), source.lastIndexOf('</script>') + 9);
async function csvDownload(page, button) {
  const promise = page.waitForEvent("download");
  await button.click();
  return fs.readFileSync(await (await promise).path(), "utf8");
}
for (const markup of ["implicit tbody", "sectioned"]) {
  for (const state of ["initial", "ascending", "descending"]) {
    for (const selected of [false, true]) {
      test(`CSV ${selected ? "selected" : "full"} (${markup}, ${state})`, async ({ page }) => {
        const header = '<tr><th>Name ↑ ↓ ↕</th><th>Note, "quoted"</th></tr>';
        const body = '<tr><td>Zulu ↑</td><td>keep comma, "quote"</td></tr><tr><td>Alpha ↓</td><td>keep two   words ↕</td></tr><tr><td>Hidden</td><td>excluded</td></tr>';
        const rows = markup === "sectioned" ? `<thead>${header}</thead><tbody>${body}</tbody>` : header + body;
        await page.route("**/csv-fixture", route => route.fulfill({ contentType: "text/html", body: `<html><head><meta charset="utf-8"><title>CSV test</title></head><body><h1>CSV test</h1><div class="wolfbbs-table-wrap"><table>${rows}</table></div>${script}</body></html>` }));
        await page.goto("/csv-fixture");
        const table = page.locator("table");
        await expect(table).toHaveAttribute("data-wolfbbs-enhanced", "1");
        const headerRow = table.locator("tr").filter({ has: page.locator("th") });
        const headerHTML = await headerRow.innerHTML();
        if (state !== "initial") await table.locator("th button").first().click();
        if (state === "descending") await table.locator("th button").first().click();
        await page.getByPlaceholder("Filter rows", { exact: true }).fill("keep");
        await expect(headerRow).toBeVisible();
        expect(await table.locator("tr").first().innerHTML()).toBe(await headerRow.innerHTML());
        if (state === "initial") expect(await headerRow.innerHTML()).toBe(headerHTML);
        if (selected) await page.getByRole("button", { name: "Select visible", exact: true }).click();
        // Wait for click feedback to settle before checking export DOM stability.
        await expect(table.locator(".wolfbbs-morph-active")).toHaveCount(0);
        const before = await table.evaluate(node => node.outerHTML);
        const button = page.getByRole("button", { name: selected ? "Export selected" : "Export CSV", exact: true });
        const csv = await csvDownload(page, button);
        const expectedHeader = '"Name ↑ ↓ ↕","Note, ""quoted"""';
        const alpha = '"Alpha ↓","keep two words ↕"';
        const zulu = '"Zulu ↑","keep comma, ""quote"""';
        expect(csv).toBe([expectedHeader, ...(state === "ascending" ? [alpha, zulu] : [zulu, alpha])].join("\n"));
        expect(await table.evaluate(node => node.outerHTML)).toBe(before);
        if (selected) {
          await table.locator("tr").filter({ hasText: "Zulu" }).click({ modifiers: ["ControlOrMeta"] });
          expect(await csvDownload(page, button)).toBe([expectedHeader, alpha].join("\n"));
        }
      });
    }
  }
}

for (const state of ["initial", "ascending", "descending"]) {
  for (const selected of [false, true]) {
    test(`CSV actual admin users page (${selected ? "selected" : "full"}, ${state})`, async ({ page }) => {
      await page.goto("/admin/login");
      await page.fill('input[name="handle"]', process.env.WOLFBBS_E2E_ADMIN_HANDLE || "sysop");
      await page.fill('input[name="password"]', process.env.WOLFBBS_E2E_ADMIN_PASSWORD || "password123");
      await page.click('button[type="submit"]');
      await page.goto("/admin/users");
      await page.getByLabel("Sysop tools", { exact: true }).check();
      const table = page.locator("table").filter({ has: page.locator("th", { hasText: "Handle" }) }).first();
      await expect(table).toHaveAttribute("data-wolfbbs-enhanced", "1");
      const toolbar = table.locator("xpath=ancestor::div[contains(@class,'wolfbbs-table-wrap')]/preceding-sibling::div[contains(@class,'wolfbbs-table-toolbar')][1]");
      const headers = ["Handle", "Status", "Role", "Verified", "Last Login", "Actions"];
      if (state !== "initial") await table.locator("th button").first().click();
      if (state === "descending") await table.locator("th button").first().click();
      await expect(table.locator("tr").first().locator("th")).toHaveCount(headers.length);
      await toolbar.getByPlaceholder("Filter rows", { exact: true }).fill("sysop");
      if (selected) await toolbar.getByRole("button", { name: "Select visible", exact: true }).click();
      const expectedRows = await table.locator("tr").evaluateAll(rows => rows.filter(row => !row.querySelector("th") && !row.classList.contains("wolfbbs-empty-row") && row.style.display !== "none").map(row => Array.from(row.querySelectorAll("td"), cell => (cell.textContent || "").replace(/\s+/g, " ").trim())));
      expect(expectedRows.length).toBeGreaterThan(0);
      // Wait for click feedback to settle before checking export DOM stability.
      await expect(table.locator(".wolfbbs-morph-active")).toHaveCount(0);
      const before = await table.evaluate(node => node.outerHTML);
      const csv = await csvDownload(page, toolbar.getByRole("button", { name: selected ? "Export selected" : "Export CSV", exact: true }));
      const quote = value => '"' + value.replace(/"/g, '""') + '"';
      expect(csv).toBe([headers, ...expectedRows].map(row => row.map(quote).join(",")).join("\n"));
      expect(await table.evaluate(node => node.outerHTML)).toBe(before);
    });
  }
}
