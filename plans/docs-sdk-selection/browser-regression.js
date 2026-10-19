async (page) => {
  const origin = new URL(page.url()).origin;
  const base = `${origin}/openapi-sdkgen`;
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`${base}/typescript/playground.html`);
  await page.waitForFunction(() => !document.querySelector('.example-item')?.disabled);
  await page.evaluate(() => {
    window.__retiredWorkerErrors = [];
    const OriginalWorker = window.Worker;
    window.Worker = class extends OriginalWorker {
      addEventListener(type, callback, options) {
        if (type === 'error') window.__retiredWorkerErrors.push(callback);
        return super.addEventListener(type, type === 'message'
          ? event => setTimeout(() => callback(event), 250) : callback, options);
      }
    };
  });
  const document = JSON.stringify({
    openapi: '3.1.0', info: { title: 'Retained source', version: '1' },
    paths: { '/health': { get: { operationId: 'health', responses: { '204': { description: 'OK' } } } } },
  });
  await page.locator('input[type=file]').setInputFiles({ name: 'retained.json', mimeType: 'application/json', buffer: Buffer.from(document) });
  await page.selectOption('#locale-desktop', 'ko');
  await page.waitForURL('**/ko/typescript/playground.html*');
  await page.locator('.target-summary').waitFor();
  for (let iteration = 0; iteration < 4; iteration++) {
    // Dispatch before route chunks complete, including reuse of the current component.
    await page.evaluate(() => {
      const select = window.document.querySelector('#sdk-desktop');
      for (const sdk of ['golang', 'typescript']) {
        select.value = sdk; select.dispatchEvent(new Event('change', { bubbles: true }));
      }
    });
    if (iteration > 0) await page.evaluate(() => window.__retiredWorkerErrors[0]?.(new Event('error')));
    await page.waitForFunction(() => window.document.querySelector('.target-summary')?.textContent.includes('TypeScript'), null, { timeout: 10000 });
    if (await page.locator('.tree-header strong').innerText() !== 'retained.json') throw new Error('Uploaded source was lost');
  }
  const downloads = [];
  const onDownload = download => downloads.push(download.suggestedFilename());
  page.on('download', onDownload);
  await page.evaluate(() => {
    window.document.querySelector('.download-all').click();
    const select = window.document.querySelector('#sdk-desktop');
    select.value = 'golang'; select.dispatchEvent(new Event('change', { bubbles: true }));
  });
  await page.waitForFunction(() => window.document.querySelector('.target-summary')?.textContent.includes('Go'), null, { timeout: 10000 });
  page.off('download', onDownload);
  if (downloads.length) throw new Error('Stale ZIP was downloaded');
  await page.goto(`${base}/typescript/playground.html`);
  await page.waitForFunction(() => !window.document.querySelector('.example-item')?.disabled);
  await page.evaluate(() => {
    window.fetch = async () => {
      await new Promise(resolve => setTimeout(resolve, 1000));
      return new Response(JSON.stringify({ openapi: '3.1.0', info: { title: 'Late source', version: '1' }, paths: {} }));
    };
  });
  await page.locator('input[type=url]').fill('https://example.test/delayed');
  await page.locator('.url-form button').click();
  await page.selectOption('#sdk-desktop', 'golang');
  await page.waitForURL('**/golang/playground.html*');
  await page.waitForTimeout(1300);
  if (await page.locator('.target-summary, .error-state, .loading-state').count()) throw new Error('Retired fetch changed the current screen');
  return { retainedSource: true, repeatedRoundTrips: 4, retiredWorkerErrorIgnored: true, staleZIPDiscarded: true, staleFetchDiscarded: true };
}
