-- Pandoc Lua filter for Memos DOCX export (embedded in the binary).
-- * A paragraph starting with U+2063 (invisible separator) is the memo meta
--   line: strip the marker and give it the "Memo Meta" paragraph style
--   (small gray, not italic — italic CJK looks bad).
-- * Horizontal rules become a thin light-gray "Memo Separator" paragraph
--   instead of pandoc's VML shape.
local MARK = "\u{2063}"

function Para(p)
  local first = p.content[1]
  if first and first.t == "Str" and first.text:sub(1, #MARK) == MARK then
    first.text = first.text:sub(#MARK + 1)
    if first.text == "" then p.content:remove(1) end
    return pandoc.Div({ pandoc.Para(p.content) }, { ["custom-style"] = "Memo Meta" })
  end
end

function HorizontalRule()
  return pandoc.Div({ pandoc.Para({ pandoc.Str("\u{200B}") }) }, { ["custom-style"] = "Memo Separator" })
end
