/**
 * Render step for the OpenCloud assets (via @resvg/resvg-js, global install).
 *
 *   opencloud-banner.png / -dark.png : rasterises the self-contained banner SVGs
 *                                      produced by gen-banner.mjs (text already
 *                                      baked to paths, so NO font is needed here).
 *   icon.svg / icon.png              : the CA / container icon - the OFFICIAL
 *                                      OpenCloud favicon flattened to a clean,
 *                                      resvg-safe SVG (solid teal #20434f tile +
 *                                      the lavender #e2baff cube mark), 512x512.
 *   opencloud-banner-logo.png        : 1600x500 textless support-thread banner -
 *                                      the official logo centred on white.
 *
 * The favicon geometry below is copied VERBATIM from the official
 * opencloud-favicon.svg (opencloud-eu/opencloud); only the svgjs wrapper + the
 * no-op prefers-color-scheme <style> are dropped so resvg renders it reliably.
 *
 * Run: node .github/assets/gen-banner.mjs && node .github/assets/gen-assets.mjs
 */
import { readFileSync, writeFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { execSync } from "node:child_process";

const require = createRequire(import.meta.url);
const { Resvg } = require(`${execSync("npm root -g").toString().trim()}/@resvg/resvg-js`);

const __dir = dirname(fileURLToPath(import.meta.url));

// ---- 1. banner PNGs (theme pair) -------------------------------------------
for (const [suffix, bg] of [["", "#ffffff"], ["-dark", "#0d1117"]]) {
  const svg = readFileSync(join(__dir, `opencloud-banner${suffix}.svg`), "utf8");
  const png = new Resvg(svg, { fitTo: { mode: "width", value: 1600 }, background: bg });
  writeFileSync(join(__dir, `opencloud-banner${suffix}.png`), png.render().asPng());
  console.log(`opencloud-banner${suffix}.png written (1600x500)`);
}

// ---- 2. CA / container icon (official favicon, flattened) -------------------
// Solid teal tile + the three official lavender cube polygons. Verbatim geometry
// from opencloud-favicon.svg. A solid tile is the house rule for the CA page, and
// the corners are rounded on the tile itself (rx/ry) — CA's own CSS only rounds
// transparent-background icons on the Black theme, so a solid edge-to-edge tile
// must bring its own rounding. Radius = Krusader's house ratio (~13.5% of edge).
const iconSvg = `<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512">
  <rect x=".02" y="0" width="512" height="512" rx="69" ry="69" fill="#20434f"/>
  <polygon points="255.98 342.75 271.89 333.57 271.89 267.12 329.08 234.1 329.08 215.78 313.18 206.6 255.6 239.84 198.83 207.06 182.93 216.24 182.93 234.56 240.12 267.58 240.12 333.59 255.98 342.75" fill="#e2baff"/>
  <polygon points="401.95 150.82 256 66.56 256 66.56 256 66.56 110.05 150.82 110.05 187.5 256 103.24 401.95 187.5 401.95 150.82" fill="#e2baff"/>
  <polygon points="401.95 324.5 256 408.76 110.06 324.5 110.06 361.17 256 445.43 256 445.43 256 445.43 401.95 361.17 401.95 324.5" fill="#e2baff"/>
</svg>
`;
writeFileSync(join(__dir, "icon.svg"), iconSvg);
writeFileSync(join(__dir, "icon.png"), new Resvg(iconSvg, { fitTo: { mode: "width", value: 512 } }).render().asPng());
console.log("icon.svg + icon.png written (512x512 solid teal tile, official cube mark)");

// ---- 3. textless support-thread banner -------------------------------------
// White 1600x500 with the official logo (mark + wordmark) centred.
{
  const BW = 1600, BH = 500, LW = 820;
  let logo = readFileSync(join(__dir, "opencloud-logo.svg"), "utf8").replace(/<\?xml[^>]*\?>\s*/, "");
  const vb = (logo.match(/viewBox="([^"]+)"/) || [])[1] || "0 0 170 35";
  const [, , vbW, vbH] = vb.split(/\s+/).map(Number);
  const LH = LW * (vbH / vbW);
  logo = logo.replace(
    /<svg\b[^>]*>/,
    `<svg x="${((BW - LW) / 2).toFixed(2)}" y="${((BH - LH) / 2).toFixed(2)}" width="${LW}" height="${LH.toFixed(2)}" viewBox="${vb}" xmlns="http://www.w3.org/2000/svg">`,
  );
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${BW}" height="${BH}" viewBox="0 0 ${BW} ${BH}">
  <rect width="${BW}" height="${BH}" fill="#ffffff"/>
  ${logo}
</svg>
`;
  writeFileSync(join(__dir, "opencloud-banner-logo.svg"), svg);
  writeFileSync(join(__dir, "opencloud-banner-logo.png"), new Resvg(svg, { fitTo: { mode: "width", value: BW }, background: "#ffffff" }).render().asPng());
  console.log("opencloud-banner-logo.svg + .png written (1600x500, textless)");
}
