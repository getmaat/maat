// Screenshots the .card element of an HTML page at 2x.
//   node lib/render-card.mjs <page.html> <out.png> <width> <height>
import { chromium } from 'playwright';

const [, , html, out, w, h] = process.argv;
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: +w, height: +h }, deviceScaleFactor: 2 });
await page.goto('file://' + html);
await page.evaluate(() => document.fonts.ready);
await page.locator('.card').screenshot({ path: out, omitBackground: true });
await browser.close();
