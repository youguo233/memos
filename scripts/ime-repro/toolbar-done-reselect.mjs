import { webkit, devices } from "playwright";
const b = await webkit.launch(); const p = await (await b.newContext(devices["iPhone 13"])).newPage();
await p.goto("file:///tmp/h/index3.html");
const st = (tag) => p.evaluate((tag) => { const tb=document.querySelector(".mbe-toolbar"); return tag+" "+JSON.stringify({open:!!tb&&!tb.hidden&&tb.isConnected&&tb.classList.contains("mbe-toolbar-docked"), hide:tb?.classList.contains("mbe-toolbar-scroll-hide"), v:tb?.dataset.mbeTb, sel:!getSelection().isCollapsed}); }, tag);
const sel = () => p.evaluate(() => { const t = document.querySelectorAll(".mbe-root > p")[1].firstChild; getSelection().setBaseAndExtent(t, 2, t, 4); });
// 1 long-press: touchstart w/o touchend
await p.dispatchEvent(".mbe-root > p:nth-child(2)", "touchstart", {touches:[]}).catch(()=>{});
await sel(); await p.setViewportSize({width:390,height:400}); await p.waitForTimeout(800); console.log(await st("1 select+kb"));
// 2 Done: blur, DOM selection kept, keyboard closes
await p.evaluate(()=>document.activeElement.blur()); await p.setViewportSize({width:390,height:664}); await p.waitForTimeout(1000); console.log(await st("2 after Done"));
// 3 re-select via long-press
await p.evaluate(()=>getSelection().removeAllRanges());
await p.dispatchEvent(".mbe-root > p:nth-child(2)", "touchstart", {touches:[]}).catch(()=>{});
await sel(); await p.setViewportSize({width:390,height:400}); await p.waitForTimeout(800); console.log(await st("3 reselect+kb"));
await b.close();
