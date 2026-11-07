import { webkit, devices } from "playwright";
const b = await webkit.launch(); const p = await (await b.newContext(devices["iPhone 13"])).newPage();
await p.goto("file:///tmp/h/index3.html");
await p.setViewportSize({width:390,height:400});
// Simulate iOS keyboard: visualViewport offset into a scrolled layout viewport.
await p.evaluate(() => { document.body.style.height = "3000px"; const fake = { offsetTop: 300, offsetLeft: 0, height: 380, width: 390, scale: 1, addEventListener: () => {}, removeEventListener: () => {} }; Object.defineProperty(window, "visualViewport", { get: () => fake, configurable: true }); });
await p.click(".mbe-root p:nth-child(2)");
await p.evaluate(() => { const t = document.querySelectorAll(".mbe-root > p")[1].firstChild; getSelection().setBaseAndExtent(t, 2, t, 6); });
await p.waitForTimeout(300);
await p.evaluate(()=>document.querySelector(".mbe-tool-text-style").click()); await p.waitForTimeout(150);
console.log(JSON.stringify(await p.evaluate(() => { const m=document.querySelector(".mbe-menu-docked:not([hidden])"); const tb=document.querySelector(".mbe-toolbar").getBoundingClientRect(); const r=m.getBoundingClientRect(); return {v:m.dataset.mbeTb, menuBottom:Math.round(r.bottom), menuTop:Math.round(r.top), tbTop:Math.round(tb.top), gap:Math.round(tb.top-r.bottom), tbX:Math.round(tb.left), mX:Math.round(r.left)}; })));
await b.close();
