import { webkit, devices } from "playwright";
for (const ios of [true, false]) {
const b = await webkit.launch(); const ctx = await b.newContext(ios ? devices["iPhone 13"] : {}); const p = await ctx.newPage();
await p.goto("file:///tmp/h/index.html"); await p.click(".mbe-root");
await p.keyboard.press("End"); await p.keyboard.press("Enter"); await p.waitForTimeout(50);
const st = async (tag) => console.log(ios?"ios":"mac", tag, JSON.stringify(await p.evaluate(() => { const s = getSelection(); const l = document.querySelectorAll(".mbe-root > p")[1]; const r = { sel: s.toString(), col: s.isCollapsed, sameEl: l === window.l, sameText: l.firstChild === window.t, prevented: window.prev, html: l.outerHTML }; window.l = l; window.t = l.firstChild; return r; })));
await p.evaluate(() => { window.prev = []; document.querySelector(".mbe-root").addEventListener("beforeinput", (e) => setTimeout(() => window.prev.push(e.inputType + ":" + e.defaultPrevented))); });
await st("empty");
await p.keyboard.insertText("哈哈"); await p.waitForTimeout(80); await st("insert1");
await p.keyboard.insertText("你好"); await p.waitForTimeout(80); await st("insert2");
await p.keyboard.type(" **b** "); await p.waitForTimeout(80); await st("bold");
console.log(await p.evaluate(() => JSON.stringify(window.ed.getMarkdown?.() ?? window.ed.markdown)));
await b.close(); }
