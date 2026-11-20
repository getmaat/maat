// Renders the demo frame by frame. terminal.html maps a time t to the screen
// state, so every frame is exact and the timing is deterministic.
//   node lib/capture-demo.mjs <work-dir> [fps]
import { chromium } from 'playwright';
import fs from 'fs';

const dir = process.argv[2], fps = +(process.argv[3] || 15);
const story = JSON.parse(fs.readFileSync(dir + '/story.json', 'utf8'));
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1200, height: 720 }, deviceScaleFactor: 1 });
await page.goto('file://' + dir + '/terminal.html');
await page.evaluate(() => document.fonts.ready);
const duration = await page.evaluate(s => window.__setup(s), story);
fs.mkdirSync(dir + '/frames', { recursive: true });
const n = Math.ceil(duration * fps);
for (let i = 0; i < n; i++) {
  await page.evaluate(t => window.__render(t), i / fps);
  await page.screenshot({ path: `${dir}/frames/${String(i).padStart(5, '0')}.png` });
}
console.log(`  ${n} frames, ${duration.toFixed(1)} s`);
await browser.close();
