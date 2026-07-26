/**
 * Generates the OpenCloud README banners (house theme-adaptive pair):
 *   opencloud-banner.svg / .png      : light 1600x500 - white bg, dark-teal wordmark
 *   opencloud-banner-dark.svg / .png : dark  1600x500 - GitHub-dark bg, lavender wordmark
 *
 * The logo + wordmark are the OFFICIAL OpenCloud brand SVGs, embedded VERBATIM
 * (never redrawn): opencloud-logo.svg (dark-teal #20434F for light backgrounds)
 * and opencloud-logo-dark.svg (lavender #E2BAFF for dark backgrounds), both taken
 * unmodified from opencloud-eu/opencloud. Only the background and the claim colour
 * flip between the two themes. The README serves the pair via <picture>.
 *
 * The cheeky claim is set in Lato (OFL, a humanist sans shared across the house
 * repos), fetched at runtime to the OS temp dir and converted to SVG paths with
 * opentype.js so the SVG is self-contained (no font needed at render time). The
 * font is NEVER committed.
 *
 * viewBox-agnostic: the logo's own viewBox is read from the file and reused, so a
 * future official-logo swap with a different viewBox keeps working.
 *
 * Deps: `npm i -g @resvg/resvg-js opentype.js`. Run:
 *   node .github/assets/gen-banner.mjs && node .github/assets/gen-assets.mjs
 */
import { readFileSync, writeFileSync, existsSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { tmpdir } from "node:os";
import { createRequire } from "node:module";
import { execSync } from "node:child_process";

const require = createRequire(import.meta.url);
const opentype = require(`${execSync("npm root -g").toString().trim()}/opentype.js`);

const __dir = dirname(fileURLToPath(import.meta.url));

// ---- content + styling -----------------------------------------------------
const CLAIM = "Your files. Your cloud. Your terms.";
// Theme-adaptive pair (house rule): light + dark, served via <picture>. Each
// theme embeds the matching OFFICIAL logo variant so the wordmark always reads.
const THEMES = [
  { suffix: "",      bg: "#ffffff", logo: "opencloud-logo.svg",      claim: "#5a5d5e" },
  { suffix: "-dark", bg: "#0d1117", logo: "opencloud-logo-dark.svg", claim: "#9aa4ad" },
];
const W = 1600, H = 500;
const LOGO_W = 760;                 // rendered wordmark width on the banner
const claimSize = 40, lineGap = 40; // gap between wordmark and claim
// ---------------------------------------------------------------------------

// Lato (OFL) for the claim - fetched at runtime, never committed.
const claimFontPath = join(tmpdir(), "opencloud-Lato-Regular.ttf");
if (!existsSync(claimFontPath)) {
  const r = await fetch("https://github.com/google/fonts/raw/main/ofl/lato/Lato-Regular.ttf");
  if (!r.ok) throw new Error(`claim font fetch ${r.status}`);
  writeFileSync(claimFontPath, Buffer.from(await r.arrayBuffer()));
}
const claimFont = opentype.parse(readFileSync(claimFontPath));

const claimW = claimFont.getAdvanceWidth(CLAIM, claimSize);
const cEm = (s) => s / claimFont.unitsPerEm;
const claimAsc = claimFont.ascender * cEm(claimSize);
const claimDesc = -claimFont.descender * cEm(claimSize);

for (const t of THEMES) {
  // Embed the official logo verbatim; read its own viewBox (viewBox-agnostic).
  let logo = readFileSync(join(__dir, t.logo), "utf8").replace(/<\?xml[^>]*\?>\s*/, "");
  const vb = (logo.match(/viewBox="([^"]+)"/) || [])[1] || "0 0 170 35";
  const [, , vbW, vbH] = vb.split(/\s+/).map(Number);
  const LW = LOGO_W;
  const LH = LW * (vbH / vbW);

  // Vertically centre the group [logo] + lineGap + [claim].
  const blockH = LH + lineGap + claimAsc + claimDesc;
  const top = (H - blockH) / 2;
  const LX = (W - LW) / 2;
  const LY = top;
  const claimBaseline = top + LH + lineGap + claimAsc;
  const claimX = (W - claimW) / 2;

  logo = logo.replace(
    /<svg\b[^>]*>/,
    `<svg x="${LX.toFixed(2)}" y="${LY.toFixed(2)}" width="${LW}" height="${LH.toFixed(2)}" viewBox="${vb}" xmlns="http://www.w3.org/2000/svg">`,
  );

  const claimPath = claimFont.getPath(CLAIM, claimX, claimBaseline, claimSize).toPathData(2);

  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}" role="img" aria-label="OpenCloud">
  <rect width="${W}" height="${H}" fill="${t.bg}"/>
  ${logo}
  <path d="${claimPath}" fill="${t.claim}"/>
</svg>
`;
  writeFileSync(join(__dir, `opencloud-banner${t.suffix}.svg`), svg);
  console.log(`opencloud-banner${t.suffix}.svg written (logo ${LW}x${LH.toFixed(0)}, claim ${Math.round(claimW)}px)`);
}
console.log("now run gen-assets.mjs for the PNGs + CA icon + support banner");
