import { webkit, devices } from "playwright";
const b = await webkit.launch(); const p = await (await b.newContext(devices["iPhone 13"])).newPage();
const logs=[]; p.on("console", m => logs.push(m.text()));
await p.goto("file:///tmp/h/index.html?imedebug=1"); await p.click(".mbe-root");
await p.keyboard.press("End"); await p.keyboard.press("Enter"); await p.waitForTimeout(50);
await p.evaluate(async () => {
  const r=document.querySelector(".mbe-root"); const l=r.children[1];
  r.dispatchEvent(new CompositionEvent("compositionstart",{bubbles:true,data:""}));
  const t=l.firstChild; t.data="哈"; l.querySelector("br")?.remove();
  getSelection().setBaseAndExtent(t,0,t,1);
  r.dispatchEvent(new InputEvent("input",{bubbles:true,inputType:"insertCompositionText",data:"哈",isComposing:true}));
  getSelection().setBaseAndExtent(t,1,t,1);
  r.dispatchEvent(new CompositionEvent("compositionend",{bubbles:true,data:"哈"}));
  await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));
  window.res = { col: getSelection().isCollapsed, off: getSelection().anchorOffset, md: ed.getMarkdown() };
});
console.log(JSON.stringify(await p.evaluate(()=>window.res)));
console.log(logs.filter(x=>/setBaseAndExtent\(#text\("哈"\),0|kept browser caret|guard/.test(x)).join("\n"));
await b.close();
