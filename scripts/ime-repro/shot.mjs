import { webkit, devices } from "playwright";
const b = await webkit.launch();
for (const [name, ctxOpt] of [["iphone", devices["iPhone 13"]], ["desktop", { viewport: { width: 800, height: 600 } }]]) {
  const p = await (await b.newContext(ctxOpt)).newPage();
  await p.goto("file:///tmp/h/index2.html");
  await p.click(".mbe-root p:nth-child(2)");
  await p.evaluate(() => { const t = document.querySelectorAll(".mbe-root > p")[1].firstChild; getSelection().setBaseAndExtent(t, 2, t, 8); });
  await p.waitForTimeout(300);
  const info = await p.evaluate(() => { const tb = document.querySelector(".mbe-toolbar"); const r = tb && !tb.hidden ? tb.getBoundingClientRect() : null; const s = getSelection().getRangeAt(0).getBoundingClientRect(); const btn = tb?.querySelector(".mbe-tool")?.getBoundingClientRect(); return { docked: tb?.classList.contains("mbe-toolbar-docked"), tb: r && [r.top, r.bottom, r.left, r.width].map(Math.round), sel: [s.top, s.bottom].map(Math.round), btn: btn && [btn.width, btn.height].map(Math.round), vh: visualViewport.height }; });
  console.log(name, JSON.stringify(info));
  await p.screenshot({ path: `/workspace/toolbar-${name}.png` });
}
await b.close();
