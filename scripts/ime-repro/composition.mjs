import { chromium, webkit } from "playwright";
for (const [name, bt, opts] of [["chromium", chromium, { executablePath: "/usr/bin/google-chrome" }], ["webkit", webkit, {}]]) {
  const b = await bt.launch(opts);
  const p = await b.newPage();
  p.on("pageerror", (e) => console.log(name, "PAGEERROR", e.message));
  await p.goto("file:///tmp/h/index.html");
  await p.click(".mbe-root");
  await p.keyboard.press("End"); await p.keyboard.press("Enter");
  await p.evaluate(() => { window.line = document.querySelectorAll(".mbe-root > p")[1]; });
  const info = (label) => p.evaluate((label) => { const s = getSelection(); const ps = document.querySelectorAll(".mbe-root > p"); const out = `${label}: sel="${s.toString()}" collapsed=${s.isCollapsed} sameEl=${ps[1] === window.line} sameText=${ps[1]?.firstChild === window.txt}`; window.line = ps[1]; window.txt = ps[1]?.firstChild; return out + ` html=${ps[1]?.outerHTML}`; }, label);
  if (name === "chromium") {
    const c = await p.context().newCDPSession(p);
    for (const t of ["h", "hh", "hhh"]) await c.send("Input.imeSetComposition", { text: t, selectionStart: t.length, selectionEnd: t.length });
    await c.send("Input.insertText", { text: "哈哈哈" });
  } else {
    await p.keyboard.insertText("哈哈哈");
  }
  await p.waitForTimeout(150); console.log(name, await info("commit"));
  await p.keyboard.insertText("红"); await p.waitForTimeout(100); console.log(name, await info("next"));
  await p.keyboard.type(" abc"); await p.waitForTimeout(100); console.log(name, await info("english"));
  await p.keyboard.type(" **b**"); await p.waitForTimeout(100); console.log(name, await info("bold"));
  console.log(name, "markdown:", JSON.stringify(await p.evaluate(() => window.ed.getMarkdown?.() ?? window.ed.markdown)));
  await b.close();
}
