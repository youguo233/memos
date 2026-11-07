import { webkit, devices } from "playwright";
const b = await webkit.launch(); const p = await (await b.newContext(devices["iPhone 13"])).newPage();
await p.goto("file:///tmp/h/index3.html");
await p.setViewportSize({width:390,height:380}); // keyboard open
await p.click(".mbe-root p:nth-child(2)");
await p.evaluate(() => { const t = document.querySelectorAll(".mbe-root > p")[1].firstChild; getSelection().setBaseAndExtent(t, 2, t, 6); });
await p.waitForTimeout(300);
for (const tool of [".mbe-tool-text-style", ".mbe-tool-list"]) {
  await p.tap(tool); await p.waitForTimeout(150);
  const info = await p.evaluate(() => { const m=[...document.querySelectorAll(".mbe-menu")].find(e=>!e.hidden); const tb=document.querySelector(".mbe-toolbar").getBoundingClientRect(); const r=m.getBoundingClientRect(); return {v:m.dataset.mbeTb, top:Math.round(r.top), bottom:Math.round(r.bottom), tbTop:Math.round(tb.top), maxH:m.style.maxHeight, scroll:m.scrollHeight>m.clientHeight, vh:visualViewport.height, active:document.activeElement.className}; });
  console.log(tool, JSON.stringify(info));
  if (tool === ".mbe-tool-text-style") {
    await p.screenshot({path:"/workspace/toolbar-menu-up.png"});
    const rows = await p.$$(".mbe-menu-docked:not([hidden]) .mbe-menu-item");
    await rows[1].scrollIntoViewIfNeeded(); await rows[1].tap(); await p.waitForTimeout(200);
    console.log("after pick:", await p.evaluate(()=>({md:ed.getMarkdown().split("\n").slice(0,4), sel:getSelection().toString(), active:document.activeElement.className, tb:!document.querySelector(".mbe-toolbar").hidden})));
  } else { await p.tap(tool); }
}
await b.close();
