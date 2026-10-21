// Shared helpers use only documented Ego Page/TaskSpace/CDP APIs.
const __ego = {
  documentClip(rect, viewport, scale, dpr) {
    return {...rect, x: rect.x + (viewport.pageX || 0), y: rect.y + (viewport.pageY || 0), scale: scale / dpr};
  },
  async pick(task, requested) {
    const tabs = await task.tabs();
    const managed = tabs.filter(t => t.label);
    let tab = managed.find(t => t.label === requested);
    if (!tab) tab = managed.find(t => t.active) || managed[0];
    if (!tab) throw new Error('No managed page remains; call ego_lite_new_page or adopt an existing tab.');
    return { page: task.page(tab.label), recoveredFrom: tab.label === requested ? null : requested };
  },
  async frames(page) {
    const { frameTree } = await page.cdp('Page.getFrameTree');
    const out = [];
    const walk = (node, parent = null) => {
      if (!node) return;
      out.push({ id: node.frame.id, parentId: parent, url: node.frame.url, name: node.frame.name || '', main: parent === null });
      for (const child of node.childFrames || []) walk(child, node.frame.id);
    };
    walk(frameTree);
    return out;
  },
  async evaluate(page, expression, frame) {
    const source = `(async()=>{const value=(${expression}\n);return typeof value==='function'?await value():await value;})()`;
    if (!frame || frame === 'main') return await page.evaluate(source);
    const frames = await this.frames(page);
    const target = frames.find(f => f.id === frame || f.name === frame);
    if (!target) throw new Error('Unknown frame: ' + frame + '; call ego_lite_frames.');
    if (target.main) return await page.evaluate(source);
    await page.cdp('Runtime.enable');
    const events = await page.events();
    const contexts = events.filter(e => e?.method === 'Runtime.executionContextCreated').map(e => e.params.context);
    const context = contexts.find(c => c.auxData?.frameId === target.id && c.auxData?.isDefault);
    if (!context) throw new Error('Frame main execution context unavailable (possibly an out-of-process iframe). Use native snapshot refs for actions, or navigate its URL in a separate managed page.');
    const result = await page.cdp('Runtime.evaluate', { expression: source, contextId: context.id, awaitPromise: true, returnByValue: true });
    if (result.exceptionDetails) throw new Error(result.exceptionDetails.exception?.description || result.exceptionDetails.text);
    return result.result?.value ?? null;
  },
  // This DOM resolver deliberately handles CSS, text, role, href and XPath.
  // Snapshot refs stay with Ego's native resolver and are never guessed.
  resolveInPage({ selector, nth, visible, scroll = false, tag = null }) {
    let sel = selector.trim();
    const terminal = sel.match(/\s*>>\s*nth=(-?\d+)\s*$/);
    if (terminal) { if (nth == null) nth = Number(terminal[1]); if (nth < -1) throw new Error('nth must be -1 or non-negative'); sel = sel.slice(0, terminal.index).trim(); }
    if (/^(?:@\d+|ref=\d+)$/.test(sel)) throw new Error('box/frame/nth filtering cannot resolve snapshot refs; use the native action directly or a CSS/text locator.');
    if (sel.startsWith('loc=role:')) throw new Error('Role locators must use the native Ego action without frame/nth/visible filters. Use CSS/text/href/XPath for explicit DOM filtering.');
    const roots = [document];
    const all = [];
    for (let i = 0; i < roots.length; i++) for (const el of roots[i].querySelectorAll('*')) { all.push(el); if (el.shadowRoot) roots.push(el.shadowRoot); }
    let nodes;
    if (sel.startsWith('text=')) {
      let text = sel.slice(5); const exact = /^(["']).*\1$/.test(text);
      if (exact) text = text.slice(1, -1);
      const norm = s => s.replace(/\s+/g, ' ').trim();
      const matches = el => exact ? norm(el.textContent || '') === text : norm(el.textContent || '').toLowerCase().includes(text.toLowerCase());
      nodes = all.filter(el => matches(el) && ![...el.children].some(matches));
    } else if (sel.startsWith('loc=href:')) {
      const href = sel.slice(9); nodes = all.filter(el => el.tagName === 'A' && (el.getAttribute('href') === href || el.href === href));
    } else if (sel.startsWith('loc=role:')) {
      throw new Error('Role locators must use the native Ego action without frame/nth/visible filters. For explicit DOM filtering or box, use CSS/text/href/XPath.');
    } else if (sel.startsWith('xpath=')) {
      const result = document.evaluate(sel.slice(6), document, null, XPathResult.ORDERED_NODE_SNAPSHOT_TYPE, null);
      nodes = Array.from({length: result.snapshotLength}, (_, i) => result.snapshotItem(i)).filter(x => x instanceof Element);
    } else {
      sel = sel.replace(/^(?:loc=css:|css=)/, '');
      nodes = roots.flatMap(root => [...root.querySelectorAll(sel)]);
    }
    const isVisible = el => { const r = el.getBoundingClientRect(); const s = getComputedStyle(el); return r.width > 0 && r.height > 0 && s.display !== 'none' && s.visibility !== 'hidden'; };
    if (visible === true) nodes = nodes.filter(isVisible);
    else if (visible === false) nodes = nodes.filter(el => !isVisible(el));
    const matched = nodes.length;
    if (nth != null) nodes = nodes[nth < 0 ? nodes.length + nth : nth] ? [nodes[nth < 0 ? nodes.length + nth : nth]] : [];
    if (nodes.length !== 1) throw new Error(`Selector matched ${nodes.length} elements (before nth: ${matched}); specify nth/visible or use a more precise locator.`);
    const el = nodes[0];
    if (scroll) el.scrollIntoView({ block: 'center', inline: 'center', behavior: 'instant' });
    if (tag) el.setAttribute('data-zero-ego-target', tag);
    const r = el.getBoundingClientRect();
    return { matched, selected: 1, visible: isVisible(el), tag: el.tagName.toLowerCase(), rect: { x: r.x, y: r.y, width: r.width, height: r.height }, files: el.files ? [...el.files].map(f => ({ name: f.name, size: f.size })) : null };
  },
  async resolve(page, options) {
    const expression = `(function ${this.resolveInPage.toString()})(${JSON.stringify(options)})`;
    return await this.evaluate(page, expression, options.frame);
  },
  async fingerprint(page) {
    return await page.evaluate(() => {
      const root = document.body || document.documentElement;
      const text = (root?.innerText || '').slice(0, 100000);
      const controls = [...document.querySelectorAll('input,select,textarea,[aria-expanded],[aria-selected],dialog')].map(e => [e.value, e.checked, e.getAttribute('aria-expanded'), e.getAttribute('aria-selected'), e.open]);
      const source = text + JSON.stringify(controls);
      let hash = 2166136261;
      for (let i = 0; i < source.length; i++) hash = Math.imul(hash ^ source.charCodeAt(i), 16777619);
      return { url: location.href, hash: hash >>> 0 };
    });
  },
  async action(page, method, args) {
    let before = null;
    try { before = await this.fingerprint(page); } catch {}
    const receipt = await page[method](...args);
    const result = { receipt: receipt ?? null, noVisibleChange: null, observation: 'unknown' };
    if (receipt?.dialog || receipt?.popups?.length) { result.noVisibleChange = false; result.observation = 'receipt'; return result; }
    try {
      const after = await this.fingerprint(page);
      if (before) { result.noVisibleChange = before.url === after.url && before.hash === after.hash; result.observation = 'immediate DOM heuristic; not a success assertion'; }
    } catch (e) { result.observationError = String(e.message).split('\n')[0]; }
    return result;
  }
};
