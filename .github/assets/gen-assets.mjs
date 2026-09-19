/**
 * Renders the OpenCloud assets with @resvg/resvg-js (global install).
 *
 *   opencloud-banner.png / -dark.png : the banner SVGs from gen-banner.mjs, whose
 *                                      text is already paths, so no font is needed.
 *   icon.svg / icon.png              : the CA and container icon, the official
 *                                      OpenCloud favicon flattened to a resvg-safe
 *                                      SVG (solid teal #20434f tile, lavender
 *                                      #e2baff cube mark), 512x512.
 *   opencloud-banner-logo.png        : 1600x500 textless support-thread banner,
 *                                      the official logo centred on white.
 *
 * The favicon geometry is copied from the official opencloud-favicon.svg
 * (opencloud-eu/opencloud); only the svgjs wrapper and the no-op
 * prefers-color-scheme <style> are dropped so resvg renders it reliably.
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

for (const [suffix, bg] of [["", "#ffffff"], ["-dark", "#0d1117"]]) {
  const svg = readFileSync(join(__dir, `opencloud-banner${suffix}.svg`), "utf8");
  const png = new Resvg(svg, { fitTo: { mode: "width", value: 1600 }, background: bg });
  writeFileSync(join(__dir, `opencloud-banner${suffix}.png`), png.render().asPng());
  console.log(`opencloud-banner${suffix}.png written (1600x500)`);
}

// The CA and container icon: a solid teal tile with the three official cube
// polygons. The tile rounds its own corners because CA's CSS only rounds icons
// with a transparent background on the Black theme; the radius is the house
// ratio of about 13.5% of the edge.
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

// Textless support-thread banner: the official logo centred on white.
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
