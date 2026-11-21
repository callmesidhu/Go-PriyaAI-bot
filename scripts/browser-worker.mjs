// Persistent, deliberately conservative Playwright worker for PRIYA AI.
// Start Chromium manually with --remote-debugging-port=9222, log in yourself, then run this worker.
// It never reads cookies, passwords, or authentication tokens and stops on any challenge.
import { chromium } from 'playwright';

const cdp = process.env.PRIYA_CDP_ENDPOINT || 'http://127.0.0.1:9222';
const api = process.env.PRIYA_API_URL || 'http://127.0.0.1:8080';
const token = process.env.PRIYA_ADMIN_TOKEN;
const poll = Math.max(Number(process.env.PRIYA_POLL_MS || 45000), 15000);
const headers = { 'content-type': 'application/json', authorization: `Bearer ${token}` };
let seen = new Set();

async function state(kind, reason = '') {
  await fetch(`${api}/api/browser/state`, { method: 'POST', headers, body: JSON.stringify({ kind, reason }) });
}
async function run() {
  const browser = await chromium.connectOverCDP(cdp);
  const pages = browser.contexts().flatMap(c => c.pages());
  const page = pages.find(p => p.url().includes('instagram.com')) || await browser.contexts()[0].newPage();
  await page.goto('https://www.instagram.com/direct/inbox/', { waitUntil: 'domcontentloaded' });
  if (await page.locator('input[name="username"], text=/challenge|suspicious login|security code/i').count()) {
    await state('paused', 'Instagram requires manual login or security review'); return;
  }
  await state('ready');
  setInterval(async () => {
    try {
      if (await page.locator('text=/challenge|suspicious login|security code/i').count()) { await state('paused', 'Instagram security challenge detected'); return; }
      // UI selectors are intentionally visible-DOM only. Review after Instagram UI changes.
      const unread = page.locator('[role="main"] a').filter({ has: page.locator('[aria-label*="unread" i]') });
      for (let i = 0; i < await unread.count(); i++) {
        const row = unread.nth(i); const href = await row.getAttribute('href'); const label = await row.innerText();
        if (!href || seen.has(href)) continue; seen.add(href);
        await row.click(); await page.waitForTimeout(500);
        const text = await page.locator('[role="main"] [dir="auto"]').last().innerText().catch(() => '');
        if (!text) continue;
        await fetch(`${api}/api/browser/incoming`, { method:'POST', headers, body:JSON.stringify({ id:href+':'+text, sender_id:href, username:label.split('\n')[0], text }) });
      }
    } catch (err) { await state('paused', `Browser worker error: ${err.message}`); }
  }, poll);
}
run().catch(err => { console.error(err); process.exitCode = 1; });
