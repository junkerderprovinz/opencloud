/**
 * Generates the OpenCloud README banners (house theme-adaptive pair):
 *   opencloud-banner.svg / .png      : light 1600x500 - white bg
 *   opencloud-banner-dark.svg / .png : dark  1600x500 - GitHub-dark bg
 *
 * The official OpenCloud logo is a combined mark+wordmark lockup. To match the
 * house banner layout (a prominent mark + the wordmark at the same size as every
 * other repo's name), it is split into its mark paths (the first 3, the hexagon)
 * and its wordmark paths (the remaining 8, "OpenCloud") - both VERBATIM from the
 * official SVG, never redrawn. The mark is rendered a little larger than the
 * wordmark; the wordmark is sized to the house name height. Each theme uses the
 * matching OFFICIAL colour variant (light = teal opencloud-logo.svg, dark =
 * lavender opencloud-logo-dark.svg), so nothing is recoloured. The claim is Lato
 * (OFL) in the standard grey, converted to paths so the SVG needs no font.
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
const groot = execSync("npm root -g").toString().trim();
const opentype = require(`${groot}/opentype.js`);
const { Resvg } = require(`${groot}/@resvg/resvg-js`);

const __dir = dirname(fileURLToPath(import.meta.url));

// ---- content + sizing ------------------------------------------------------
const CLAIM = "Your files. Your cloud. Your terms.";
const W = 1600, H = 500;
const WORD_H = 120;      // wordmark height in px - the house name size
const MARK_H = 188;      // mark a little larger than the wordmark (jdp)
const GAP = 46;          // gap between mark and wordmark
const claimSize = 40, lineGap = 34;
const THEMES = [
  { suffix: "",      bg: "#ffffff", logo: "opencloud-logo.svg",      claim: "#5a5d5e" },
  { suffix: "-dark", bg: "#0d1117", logo: "opencloud-logo-dark.svg", claim: "#9aa4ad" },
];
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

// Split the official lockup into mark + wordmark by X POSITION (the paths are NOT
// ordered mark-then-wordmark; the hexagon paths sit in the middle of the list).
// The mark occupies the left ~16% of the viewBox (x 0..27 of 170); everything
// further right is the wordmark. Measure each group's tight bbox via resvg.
function parseLogo(file) {
  const raw = readFileSync(join(__dir, file), "utf8");
  const vb = (raw.match(/viewBox="([^"]+)"/) || [, "0 0 170 35"])[1];
  const vw = Number(vb.split(/\s+/)[2]);
  // Match every drawable shape, not just <path>: the "l" in the wordmark is a
  // thin <rect>, which a path-only match silently drops (renders "OpenC oud").
  const paths = raw.match(/<(?:path|rect|circle|ellipse|line|polygon|polyline)\b[^>]*?\/?>/g) || [];
  const bbox = (inner) =>
    new Resvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="${vb}">${inner}</svg>`,
      { background: "rgba(0,0,0,0)" }).innerBBox();
  const marks = [], words = [];
  for (const p of paths) {
    const b = bbox(p);
    ((b.x + b.width / 2) < vw * 0.19 ? marks : words).push(p);
  }
  const mark = marks.join(""), word = words.join("");
  return { vb, mark, word, markBB: bbox(mark), wordBB: bbox(word) };
}

// Embed a path group cropped to its bbox at (x,y,w,h) via a nested <svg viewBox>.
function place(inner, bb, x, y, h) {
  const w = h * (bb.width / bb.height);
  return {
    w,
    svg: `<svg x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" viewBox="${bb.x} ${bb.y} ${bb.width} ${bb.height}" xmlns="http://www.w3.org/2000/svg">${inner}</svg>`,
  };
}

for (const t of THEMES) {
  const L = parseLogo(t.logo);
  const markW = MARK_H * (L.markBB.width / L.markBB.height);
  const wordW = WORD_H * (L.wordBB.width / L.wordBB.height);

  // Group = [mark] GAP [wordmark], vertically centred on each other; claim below.
  const rowH = Math.max(MARK_H, WORD_H);
  const groupW = markW + GAP + wordW;
  const blockH = rowH + lineGap + claimAsc + claimDesc;
  const top = (H - blockH) / 2;
  const startX = (W - groupW) / 2;

  const markY = top + (rowH - MARK_H) / 2;
  const wordY = top + (rowH - WORD_H) / 2;
  const mark = place(L.mark, L.markBB, startX, markY, MARK_H);
  const word = place(L.word, L.wordBB, startX + markW + GAP, wordY, WORD_H);

  const claimBaseline = top + rowH + lineGap + claimAsc;
  const claimX = (W - claimW) / 2;
  const claimPath = claimFont.getPath(CLAIM, claimX, claimBaseline, claimSize).toPathData(2);

  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}" role="img" aria-label="OpenCloud">
  <rect width="${W}" height="${H}" fill="${t.bg}"/>
  ${mark.svg}
  ${word.svg}
  <path d="${claimPath}" fill="${t.claim}"/>
</svg>
`;
  writeFileSync(join(__dir, `opencloud-banner${t.suffix}.svg`), svg);
  console.log(`opencloud-banner${t.suffix}.svg written (mark ${MARK_H}px, wordmark ${WORD_H}px, claim ${Math.round(claimW)}px)`);
}
console.log("now run gen-assets.mjs for the PNGs");
