const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

(async () => {
  const baseURL = process.env.UI_BASE_URL;
  if (!baseURL) throw new Error('Run through TestFrontendBrowser with UI_SMOKE=1.');
  const out = path.resolve('.impeccable/review');
  fs.mkdirSync(out, { recursive: true });
  const browser = await chromium.launch({ channel: process.env.UI_BROWSER || 'msedge', headless: true });
  try {
    const context = await browser.newContext({ baseURL, ignoreHTTPSErrors: true, viewport: { width: 1440, height: 1000 } });
    // External widget is omitted only in this browser test; the server uses a test double.
    await context.route('https://challenges.cloudflare.com/**', route => route.abort());
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    async function fits() {
      assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'horizontal overflow');
    }
    async function shot(name) {
      await page.evaluate(() => document.fonts.ready);
      await fits();
      await page.screenshot({ path: path.join(out, name + '.png'), fullPage: true, animations: 'disabled' });
    }
    await page.goto('/');
    assert.equal(await page.locator('h1').innerText(), 'Kata-kata Lintang, ditulis oleh penuturnya.');
    assert.equal(await page.locator('.entry').count(), 3);
    const style = await page.locator('body').evaluate(el => getComputedStyle(el).backgroundColor);
    assert.equal(style, 'rgb(10, 19, 48)');
    await shot('desktop');
    await page.setViewportSize({ width: 390, height: 844 });
    await shot('mobile');
    await page.getByRole('button', { name: 'Menu', exact: true }).click();
    assert.equal(await page.getByRole('button', { name: 'Menu', exact: true }).getAttribute('aria-expanded'), 'true');
    await shot('mobile-menu');
    await page.keyboard.press('Escape');
    assert.equal(await page.getByRole('button', { name: 'Menu', exact: true }).getAttribute('aria-expanded'), 'false');
    for (const width of [320, 768, 1280, 1600]) {
      await page.setViewportSize({ width, height: 900 });
      await fits();
    }
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.getByRole('searchbox', { name: 'Cari kata', exact: true }).fill('kata-uji-1');
    await page.locator('.hero').getByRole('button', { name: 'Cari kata' }).click();
    await page.waitForURL('**/cari?**');
    assert((await page.locator('.entry').first().innerText()).includes('kata-uji-1'));
    await shot('search');
    await page.locator('.entry a').first().click();
    await shot('detail');
    await page.goto('/cari?q=zzzzzzz');
    assert(await page.getByText('Belum ada kata yang ditemukan.').isVisible());
    await shot('empty');
    await page.goto('/kirim');
    await page.locator('#headword').fill('https://invalid.example');
    await page.locator('#gloss').fill('Arti buatan untuk pengujian');
    await page.getByRole('button', { name: 'Kirim untuk ditinjau' }).click();
    await page.getByRole('alert').waitFor({ state: 'visible' });
    assert.equal(await page.locator('#headword').inputValue(), 'https://invalid.example');
    await shot('submission-error');
    await page.locator('#headword').fill('kata-uji-browser');
    await page.locator('#contact').fill('private@example.invalid');
    await page.getByRole('button', { name: 'Kirim untuk ditinjau' }).click();
    await page.getByRole('status').waitFor({ state: 'visible' });
    assert((await page.getByRole('status').innerText()).includes('akan ditinjau'));
    await shot('submission-success');
    await page.goto('/admin/login');
    await shot('login');
    await page.getByLabel('Email', { exact: true }).fill('admin@example.invalid');
    await page.getByLabel('Password', { exact: true }).fill('password-uji-bukan-rahasia');
    await page.getByRole('button', { name: 'Masuk', exact: true }).click();
    await page.waitForURL('**/admin/antrean');
    await shot('queue');
    await page.getByRole('link', { name: /kata-uji-browser/ }).click();
    await shot('review');
    await page.getByRole('button', { name: 'Terima', exact: true }).click();
    await page.waitForURL('**/admin/antrean');
    await page.goto('/admin/pengguna');
    await shot('users');
    await page.getByRole('button', { name: 'Keluar', exact: true }).click();
    await page.goto('/cari?q=kata-uji-browser');
    await page.getByRole('link', { name: 'kata-uji-browser', exact: true }).click();
    assert(!(await page.locator('main').innerText()).includes('private@example.invalid'));
    assert.deepEqual(errors, []);
    const noJS = await browser.newContext({ baseURL, ignoreHTTPSErrors: true, javaScriptEnabled: false, viewport: { width: 390, height: 844 } });
    const fallback = await noJS.newPage();
    await fallback.goto('/');
    assert(await fallback.getByRole('navigation', { name: 'Navigasi utama' }).isVisible());
    await fallback.getByRole('searchbox', { name: 'Cari kata', exact: true }).fill('kata-uji-1');
    await fallback.locator('.hero').getByRole('button', { name: 'Cari kata' }).click();
    await fallback.waitForURL('**/cari?**');
    assert(await fallback.getByRole('link', { name: 'kata-uji-1', exact: true }).isVisible());
    await noJS.close();
    await context.close();
    console.log('PASS: responsive UI, menu, search, detail, empty/error/success, login, moderation, privacy, no-JS search.');
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exit(1); });
