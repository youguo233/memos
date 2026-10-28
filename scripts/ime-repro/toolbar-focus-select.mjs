import { webkit, devices } from "playwright";
const b = await webkit.launch(); const p = await (await b.newContext(devices["iPhone 13"])).newPage();
await p.goto("file:///tmp/h/index3.html");
const st = (tag) => p.evaluate((tag) => { const tb=document.querySelector(".mbe-toolbar"); return tag+" "+JSON.stringify({open:!!tb&&!tb.hidden&&tb.isConnected&&tb.classList.contains("mbe-toolbar-docked"), hideCls:tb?.classList.contains("mbe-toolbar-scroll-hide"), active:document.activeElement.className}); }, tag);
for (const mode of ["select-then-focus","select-no-focus","dblclick"]) {
  await p.evaluate(()=>{document.activeElement.blur(); getSelection().removeAllRanges();}); await p.waitForTimeout(200);
  console.log(await st(mode+" blurred"));
  if (mode==="dblclick") await p.dblclick(".mbe-root > p:nth-child(2)");
  else await p.evaluate((m) => { const t = document.querySelectorAll(".mbe-root > p")[1].firstChild; getSelection().setBaseAndExtent(t, 2, t, 8); if (m==="select-then-focus") setTimeout(()=>document.querySelector(".mbe-root").focus({preventScroll:true}),50); }, mode);
  await p.waitForTimeout(80); console.log(await st(mode+" 80ms"));
  await p.setViewportSize({width:390,height:400}); await p.waitForTimeout(800); console.log(await st(mode+" after kb"));
  await p.setViewportSize({width:390,height:664});
}
await b.close();
