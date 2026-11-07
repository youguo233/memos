# IME / iOS editor repro (Playwright)

Manual regression scripts for the @usememos/editor patch (web/patches).

    cd scripts/ime-repro
    NODE_PATH=../../web/node_modules npx esbuild main.js --bundle --outfile=out.js --format=iife --loader:.css=empty
    npm i playwright && npx playwright install webkit
    node ios-insert.mjs      # iPhone UA: beforeinput insertText not prevented, element kept, caret collapsed
    node ios-shortcuts.mjs   # iPhone UA: "- " / "## " shortcuts still work
    node composition.mjs     # Chromium (CDP Input.imeSetComposition) + WebKit

Scripts load file:///tmp/h/index.html; adjust the path to this folder's index.html.
