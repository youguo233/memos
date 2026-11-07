import { webkit, devices } from "playwright";
const b = await webkit.launch(); const p = await (await b.newContext(devices["iPhone 13"])).newPage();
await p.goto("file:///tmp/h/index2.html"); await p.click(".mbe-root p:nth-child(2)");
await p.evaluate(() => { const t = document.querySelectorAll(".mbe-root > p")[1].firstChild; getSelection().setBaseAndExtent(t, 2, t, 8); });
await p.waitForTimeout(200); await p.tap(".mbe-tool-text-style"); await p.waitForTimeout(200);
console.log(await p.evaluate(() => { const m=document.querySelector(".mbe-text-style-menu"); const r=m.getBoundingClientRect(); return JSON.stringify({hidden:m.hidden, r:[r.top,r.bottom].map(Math.round), tb: Math.round(document.querySelector(".mbe-toolbar").getBoundingClientRect().top)}); }));
await p.screenshot({ path: "/workspace/toolbar-iphone-menu.png" });
await b.close();
