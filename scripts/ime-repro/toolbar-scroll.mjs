import { webkit, devices } from "playwright";
const b = await webkit.launch(); const p = await (await b.newContext(devices["iPhone 13"])).newPage();
await p.goto("file:///tmp/h/index2.html"); await p.evaluate(()=>{document.body.style.height="3000px"}); await p.click(".mbe-root p:nth-child(2)");
await p.evaluate(() => { const t = document.querySelectorAll(".mbe-root > p")[1].firstChild; getSelection().setBaseAndExtent(t, 2, t, 8); });
await p.waitForTimeout(200);
const pos = () => p.evaluate(() => [Math.round(document.querySelector(".mbe-toolbar").getBoundingClientRect().bottom), document.documentElement.className]);
console.log(await pos()); await p.evaluate(()=>scrollTo(0,300)); await p.waitForTimeout(50); console.log(await pos());
await p.evaluate(()=>getSelection().collapseToEnd()); await p.waitForTimeout(100); console.log(await p.evaluate(()=>[document.querySelector(".mbe-toolbar").hidden, document.documentElement.className]));
await b.close();
