package httpapi

import (
	"bytes"
	"context"
	"regexp"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

var pdfHeader = regexp.MustCompile(`^%PDF-[0-9]\.[0-9]`)

// isPDF validates the document structure before an upload can replace a stored CV.
// Stateless configuration keeps uploads independent and never accesses a config directory.
func isPDF(ctx context.Context, content []byte) bool {
	tail := content[max(0, len(content)-pdfTrailerWindow):]
	if !pdfHeader.Match(content) || !bytes.Contains(tail, []byte("%%EOF")) {
		return false
	}
	conf := model.NewStatelessConfiguration()
	conf.ValidationMode = model.ValidationStrict
	conf.Optimize = false
	conf.Limits = model.ResourceLimits{
		MaxInputBytes: maxCVBytes, MaxObjectBytes: maxCVBytes, MaxStreamBytes: maxCVBytes,
		MaxDecodeBytes: 32 << 20, MaxImagePixels: 20_000_000, MaxImageBytes: 64 << 20,
		MaxObjectCount: 20_000, MaxObjectStreamCount: 20_000, MaxObjectStreamFirst: 1 << 20,
		MaxXRefEntries: 20_000, MaxRecursionDepth: 50,
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	document, err := api.ReadAndValidate(ctx, bytes.NewReader(content), conf)
	if err != nil {
		return false
	}
	// No repair is written back to the uploaded bytes, so don't accept a document
	// whose validation depended on repairing or skipping a structural violation.
	return document.PageCount > 0 && document.ValidationReport().Empty() && hasOriginalXRef(document, content)
}

// Strict validation can reconstruct a missing cross-reference section without a
// notice. Require the parser's final xref offset to identify an original section
// or an xref-stream object, and reject its recorded offset repairs.
func hasOriginalXRef(document *model.Context, content []byte) bool {
	if document.Read.RepairOffset != 0 || document.Write.OffsetPrevXRef == nil {
		return false
	}
	offset := *document.Write.OffsetPrevXRef
	if offset <= 0 || offset >= int64(len(content)) {
		return false
	}
	section := content[offset:]
	if len(section) > 4 && bytes.Equal(section[:4], []byte("xref")) && bytes.IndexByte([]byte("\x00\t\n\f\r "), section[4]) >= 0 {
		return true
	}
	for _, entry := range document.Table {
		if entry != nil && entry.Offset != nil && *entry.Offset == offset {
			_, isXRefStream := entry.Object.(types.XRefStreamDict)
			return isXRefStream
		}
	}
	return false
}
