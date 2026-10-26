package memodocx

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
)

var (
	reTblW      = regexp.MustCompile(`<w:tblW w:type="auto" w:w="0"\s*/>`)
	reTblStyle  = regexp.MustCompile(`<w:tblPr>\s*<w:tblStyle w:val="Table"\s*/>`)
	reTblLayout = regexp.MustCompile(`<w:tblLayout [^>]*/>`)
)

// polishDocx rewrites pandoc's word/document.xml so tables span the full text
// width (tblW 100%) with an autofit layout; borders, header shading and
// banding come from the "Table" style in the reference doc.
func polishDocx(in []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(in), int64(len(in)))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		if f.Name == "word/document.xml" {
			data = polishDocumentXML(data)
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: f.Modified})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func polishDocumentXML(doc []byte) []byte {
	if !reTblStyle.Match(doc) {
		return doc
	}
	doc = reTblW.ReplaceAll(doc, []byte(`<w:tblW w:type="pct" w:w="5000"/>`))
	doc = reTblLayout.ReplaceAll(doc, nil)
	return doc
}
