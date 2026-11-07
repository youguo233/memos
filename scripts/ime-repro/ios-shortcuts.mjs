import { webkit, devices } from "playwright";
const b = await webkit.launch(); const p = await (await b.newContext(devices["iPhone 13"])).newPage();
await p.goto("file:///tmp/h/index.html"); await p.click(".mbe-root");
await p.keyboard.press("End"); await p.keyboard.press("Enter");
await p.keyboard.type("- 项目"); await p.keyboard.press("Enter"); await p.keyboard.type("## 标题");
await p.waitForTimeout(100);
console.log(await p.evaluate(() => JSON.stringify(window.ed.getMarkdown())));
await b.close();
