/**
 * @date 2026/8/20
 * @description 无第三方依赖地生成最小 docx（段落文本 + 可选嵌入图片），
 *              用于把 Qwen 识图结果承载为可上传/导入的文档。
 */

package knowledge

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
)

// generateDocx builds a minimal .docx from the given text (split into paragraphs
// line-by-line) and an optional embedded image (base64-encoded). Returns the
// file bytes ready for upload.
func generateDocx(text string, imageB64 string, imageExt string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Build body XML: text paragraphs followed by an optional embedded image.
	var body strings.Builder
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		body.WriteString(xmlParagraph(xmlEscape(ln)))
	}

	imageID := ""
	mediaName := ""
	var mediaBytes []byte
	if imageB64 != "" {
		if imageExt == "png" {
			mediaName = "image1.png"
		} else {
			mediaName = "image1.jpg"
		}
		img, err := base64.StdEncoding.DecodeString(imageB64)
		if err != nil {
			return nil, fmt.Errorf("解码图片 base64 失败: %w", err)
		}
		mediaBytes = img
		imageID = "rId100"
		body.WriteString(xmlImageParagraph(imageID))
	}

	// Write fixed package parts.
	if err := writeZipEntry(zw, "[Content_Types].xml", contentTypesXML(mediaName)); err != nil {
		return nil, err
	}
	if err := writeZipEntry(zw, "_rels/.rels", rootRelsXML()); err != nil {
		return nil, err
	}
	if err := writeZipEntry(zw, "word/document.xml", documentXML(body.String())); err != nil {
		return nil, err
	}

	// Write image parts when present.
	if imageID != "" {
		rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="` + imageID + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/` + mediaName + `"/></Relationships>`
		if err := writeZipEntry(zw, "word/_rels/document.xml.rels", rels); err != nil {
			return nil, err
		}
		w, err := zw.Create("word/media/" + mediaName)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(mediaBytes); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeZipEntry(zw *zip.Writer, name, data string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(data))
	return err
}

// xmlParagraph wraps a text run. Chinese content is fine un-encoded; only XML
// special characters need escaping (handled by xmlEscape).
func xmlParagraph(text string) string {
	return `<w:p><w:r><w:t xml:space="preserve">` + text + `</w:t></w:r></w:p>`
}

// xmlImageParagraph embeds an image at the default 15cm x 12cm size. All used
// namespaces (wp/a/pic/r) are declared inline so the XML is namespace-valid.
func xmlImageParagraph(relID string) string {
	const cx = "4572000"
	const cy = "3438000"
	return `<w:p><w:r><w:drawing xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="` + cx + `" cy="` + cy + `"/><wp:docPr id="1" name="image"/><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:nvPicPr><pic:cNvPr id="1" name="image"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="` + relID + `"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="` + cx + `" cy="` + cy + `"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`
}

func documentXML(body string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body + `</w:body></w:document>`
}

func rootRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`
}

func contentTypesXML(mediaName string) string {
	var imgDefault string
	if mediaName != "" {
		ext := "jpg"
		if strings.HasSuffix(mediaName, ".png") {
			ext = "png"
		}
		imgDefault = `<Default Extension="` + ext + `" ContentType="image/` + ext + `"/>`
	}
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/>` + imgDefault + `<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
