import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";
const source = await readFile(new URL("../src/types.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } });
const { exportGeoPackage } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);

test("export sends all filters, bearer token and no pagination; downloads and releases URL", async (t) => {
  const events = [], link = { click() { events.push("click"); }, remove() { events.push("remove"); } };
  t.mock.method(globalThis, "fetch", async (url, options) => {
    assert.equal(url, "/api/v1/imagery/export/gpkg");
    assert.equal(options.headers.Authorization, "Bearer token");
    assert.deepEqual(JSON.parse(options.body), { catalog_id: "one", image_ids: ["a"], cloud_cover_lte: 0, cloud_cover_unknown: "include", acquired_from: "2026-01-01T00:00:00Z" });
    return new Response("gpkg", { headers: { "Content-Disposition": 'attachment; filename="aarde-footprints-20261020-123456.gpkg"' } });
  });
  t.mock.method(URL, "createObjectURL", (blob) => { assert.equal(blob.size, 4); return "blob:test"; });
  t.mock.method(URL, "revokeObjectURL", (url) => { assert.equal(url, "blob:test"); events.push("revoke"); });
  t.mock.method(globalThis, "setTimeout", (fn) => { fn(); });
  globalThis.document = { createElement: () => link, body: { appendChild() {} } };
  t.after(() => delete globalThis.document);
  await exportGeoPackage("token", { catalog: "one", ids: ["a"], cloudCoverLTE: 0, cloudCoverUnknown: "include", acquiredFrom: "2026-01-01T00:00:00Z" }, new AbortController().signal);
  assert.equal(link.download, "aarde-footprints-20261020-123456.gpkg");
  assert.deepEqual(events, ["click", "remove", "revoke"]);
});

test("empty results and other export errors preserve server messages", async (t) => {
  t.mock.method(globalThis, "fetch", async () => new Response(JSON.stringify({ error: { message: "No footprints to export." } }), { status: 404 }));
  await assert.rejects(exportGeoPackage("token", { catalog: "" }, new AbortController().signal), /No footprints to export/);
});

test("catalog export uses applied criteria, guards duplicate requests and exposes loading/errors", async () => {
  const app = await readFile(new URL("../src/App.tsx", import.meta.url), "utf8");
  assert.match(app, /exportGeoPackage\(token, buildSearch\(applied\)\.criteria/);
  assert.match(app, /if \(exportingRequest.current\) return/);
  assert.match(app, /disabled=\{exporting \|\| !searched \|\| loading\}/);
  assert.match(app, /Export GeoPackage/);
  assert.match(app, /exportError && <Alert severity="error"/);
});
