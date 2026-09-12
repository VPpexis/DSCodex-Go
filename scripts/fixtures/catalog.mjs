// Generates testdata/catalog-golden.json by running the upstream DSCodex
// catalog builder over testdata/catalog-cache.json.
//
// Usage:
//   git clone --depth 1 --branch v1.1.0 https://github.com/fish2lab/DSCodex /tmp/dscodex-v1.1.0
//   node scripts/fixtures/catalog.mjs /tmp/dscodex-v1.1.0
//
// The generated fixture is asserted structurally by internal/catalog tests;
// JSON key order is not significant.
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

const upstream = process.argv[2];
if (!upstream) {
  console.error("usage: node scripts/fixtures/catalog.mjs <upstream-v1.1.0-checkout>");
  process.exit(1);
}

const { buildCatalog } = await import(pathToFileURL(join(upstream, "src", "catalog.mjs")).href);
const root = new URL("../../", import.meta.url);
const cache = JSON.parse(readFileSync(new URL("testdata/catalog-cache.json", root), "utf8"));
const catalog = buildCatalog(cache);
writeFileSync(new URL("testdata/catalog-golden.json", root), `${JSON.stringify(catalog, null, 2)}\n`);
console.log("wrote testdata/catalog-golden.json");
