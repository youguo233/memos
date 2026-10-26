#!/usr/bin/env python3
"""Restyle pandoc's reference.docx for Memos DOCX export.

Usage: python3 style_reference.py in.docx out.docx
Rewrites word/styles.xml: Chinese-first typography (Arial + PingFang SC,
Consolas for code), restrained near-black headings, a grid "Table" style with a
shaded header row, gray meta line, code/quote/separator styles.
"""
import re, sys, zipfile

SRC, DST = sys.argv[1], sys.argv[2]
TEXT, MUTED, BORDER, ACCENT = "262626", "8C8C8C", "D9D9D9", "1F3A5F"
FONTS = '<w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:eastAsia="PingFang SC" w:cs="Arial"/>'
MONO = '<w:rFonts w:ascii="Consolas" w:hAnsi="Consolas" w:eastAsia="PingFang SC" w:cs="Consolas"/>'

def para(sid, name, ppr="", rpr="", based="Normal", nxt="BodyText", extra=""):
    return (f'<w:style w:type="paragraph" w:customStyle="1" w:styleId="{sid}"><w:name w:val="{name}"/>'
            f'<w:basedOn w:val="{based}"/><w:next w:val="{nxt}"/>{extra}<w:qFormat/>'
            f'<w:pPr>{ppr}</w:pPr><w:rPr>{rpr}</w:rPr></w:style>')

def heading(n, size, before, after, color=TEXT):
    return (f'<w:style w:type="paragraph" w:styleId="Heading{n}"><w:name w:val="heading {n}"/>'
            f'<w:basedOn w:val="Normal"/><w:next w:val="BodyText"/><w:link w:val="Heading{n}Char"/>'
            f'<w:uiPriority w:val="9"/><w:unhideWhenUsed/><w:qFormat/>'
            f'<w:pPr><w:keepNext/><w:keepLines/><w:spacing w:before="{before}" w:after="{after}" w:line="300" w:lineRule="auto"/>'
            f'<w:outlineLvl w:val="{n-1}"/></w:pPr>'
            f'<w:rPr>{FONTS}<w:b/><w:bCs/><w:color w:val="{color}"/><w:sz w:val="{size}"/><w:szCs w:val="{size}"/></w:rPr></w:style>')

bdr = lambda side, sz=4, color=BORDER: f'<w:{side} w:val="single" w:sz="{sz}" w:space="0" w:color="{color}"/>'
cell_borders = "".join(bdr(s) for s in ("top", "left", "bottom", "right", "insideH", "insideV"))

STYLES = {
 "Normal": '<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/>'
   f'<w:pPr><w:spacing w:after="120" w:line="348" w:lineRule="auto"/></w:pPr><w:rPr>{FONTS}<w:color w:val="{TEXT}"/>'
   '<w:sz w:val="21"/><w:szCs w:val="21"/></w:rPr></w:style>',
 "BodyText": '<w:style w:type="paragraph" w:styleId="BodyText"><w:name w:val="Body Text"/><w:basedOn w:val="Normal"/>'
   '<w:link w:val="BodyTextChar"/><w:qFormat/><w:pPr><w:spacing w:before="0" w:after="120"/></w:pPr></w:style>',
 "Compact": para("Compact", "Compact", '<w:spacing w:before="0" w:after="40" w:line="312" w:lineRule="auto"/>', based="BodyText", nxt="Compact"),
 "Title": para("Title", "Title", '<w:spacing w:before="0" w:after="360" w:line="240" w:lineRule="auto"/><w:jc w:val="left"/>'
   + f'<w:pBdr>{bdr("bottom", 12, ACCENT).replace("w:space=\"0\"", "w:space=\"8\"")}</w:pBdr>',
   f'{FONTS}<w:b/><w:color w:val="{ACCENT}"/><w:sz w:val="40"/><w:szCs w:val="40"/>'),
 "Heading1": heading(1, 26, 240, 40, ACCENT),   # memo timestamp, 13pt
 "Heading2": heading(2, 24, 240, 80),
 "Heading3": heading(3, 22, 200, 60),
 "Heading4": heading(4, 21, 160, 40),
 "Heading5": heading(5, 21, 120, 40, "595959"),
 "Heading6": heading(6, 21, 120, 40, "595959"),
 "MemoMeta": para("MemoMeta", "Memo Meta", '<w:spacing w:before="0" w:after="200"/>',
   f'<w:color w:val="{MUTED}"/><w:sz w:val="18"/><w:szCs w:val="18"/>'),
 "MemoSeparator": para("MemoSeparator", "Memo Separator",
   f'<w:pBdr>{bdr("bottom", 4)}</w:pBdr><w:spacing w:before="240" w:after="240" w:line="120" w:lineRule="exact"/>',
   '<w:sz w:val="4"/>'),
 "BlockText": para("BlockText", "Block Text",
   f'<w:pBdr>{bdr("left", 18, "BFBFBF").replace("w:space=\"0\"", "w:space=\"8\"")}</w:pBdr>'
   '<w:spacing w:before="60" w:after="60"/><w:ind w:left="240" w:right="0" w:firstLine="0"/>',
   '<w:color w:val="595959"/>', based="BodyText"),
 "SourceCode": para("SourceCode", "Source Code",
   f'<w:pBdr>{"".join(bdr(s, 4, "EDEDED").replace("w:space=\"0\"", "w:space=\"4\"") for s in ("top","left","bottom","right"))}</w:pBdr>'
   '<w:shd w:val="clear" w:color="auto" w:fill="F6F7F9"/><w:wordWrap w:val="off"/>'
   '<w:spacing w:before="0" w:after="0" w:line="264" w:lineRule="auto"/><w:ind w:left="100" w:right="100"/>',
   f'{MONO}<w:sz w:val="18"/><w:szCs w:val="18"/>', extra='<w:link w:val="VerbatimChar"/>', nxt="SourceCode"),
 "VerbatimChar": '<w:style w:type="character" w:customStyle="1" w:styleId="VerbatimChar"><w:name w:val="Verbatim Char"/>'
   f'<w:basedOn w:val="DefaultParagraphFont"/><w:rPr>{MONO}<w:color w:val="C7254E"/><w:sz w:val="19"/><w:szCs w:val="19"/>'
   '<w:shd w:val="clear" w:color="auto" w:fill="F3F4F6"/></w:rPr></w:style>',
 "Hyperlink": '<w:style w:type="character" w:styleId="Hyperlink"><w:name w:val="Hyperlink"/><w:basedOn w:val="DefaultParagraphFont"/>'
   '<w:rPr><w:color w:val="2F6FB2"/><w:u w:val="none"/></w:rPr></w:style>',
 "ImageCaption": para("ImageCaption", "Image Caption", '<w:spacing w:before="40" w:after="160"/><w:jc w:val="center"/>',
   f'<w:color w:val="{MUTED}"/><w:sz w:val="18"/>', based="BodyText"),
 "CaptionedFigure": para("CaptionedFigure", "Captioned Figure", '<w:keepNext/><w:jc w:val="center"/>', based="BodyText"),
 "Figure": para("Figure", "Figure", '<w:spacing w:before="80" w:after="80"/><w:jc w:val="center"/>', based="BodyText"),
 "Table": '<w:style w:type="table" w:default="1" w:styleId="Table"><w:name w:val="Table"/><w:basedOn w:val="TableNormal"/>'
   '<w:uiPriority w:val="59"/><w:qFormat/>'
   '<w:pPr><w:spacing w:before="0" w:after="0" w:line="300" w:lineRule="auto"/></w:pPr>'
   '<w:rPr><w:sz w:val="20"/><w:szCs w:val="20"/></w:rPr>'
   f'<w:tblPr><w:tblInd w:w="0" w:type="dxa"/><w:tblBorders>{cell_borders}</w:tblBorders>'
   '<w:tblCellMar><w:top w:w="80" w:type="dxa"/><w:left w:w="140" w:type="dxa"/><w:bottom w:w="80" w:type="dxa"/>'
   '<w:right w:w="140" w:type="dxa"/></w:tblCellMar></w:tblPr>'
   '<w:tblStylePr w:type="firstRow"><w:rPr><w:b/><w:bCs/><w:color w:val="1F2937"/></w:rPr>'
   '<w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="EEF2F6"/></w:tcPr></w:tblStylePr>'
   '<w:tblStylePr w:type="band2Horz"><w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="FAFBFC"/></w:tcPr></w:tblStylePr>'
   '</w:style>',
}

with zipfile.ZipFile(SRC) as z:
    files = {n: z.read(n) for n in z.namelist()}
s = files["word/styles.xml"].decode()
s = re.sub(r'<w:docDefaults>.*?</w:docDefaults>',
    f'<w:docDefaults><w:rPrDefault><w:rPr>{FONTS}<w:sz w:val="21"/><w:szCs w:val="21"/>'
    '<w:lang w:val="en-US" w:eastAsia="zh-CN" w:bidi="ar-SA"/></w:rPr></w:rPrDefault>'
    '<w:pPrDefault><w:pPr><w:spacing w:after="120" w:line="348" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>', s, flags=re.S)
for sid, xml in STYLES.items():
    pat = re.compile(r'<w:style [^>]*w:styleId="%s".*?</w:style>' % sid, re.S)
    s, n = pat.subn(lambda _: xml, s)
    if n == 0:
        s = s.replace("</w:styles>", xml + "</w:styles>")
# Heading char styles: drop theme colors/fonts so they follow the paragraph style.
s = re.sub(r'(<w:style [^>]*w:styleId="Heading\dChar".*?<w:rPr>).*?(</w:rPr>)', r'\1\2', s, flags=re.S)
files["word/styles.xml"] = s.encode()
with zipfile.ZipFile(DST, "w", zipfile.ZIP_DEFLATED) as z:
    for n, b in files.items():
        z.writestr(n, b)
