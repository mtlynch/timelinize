package googlevoice

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/timelinize/timelinize/timeline"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestFileImportSkipsNonHTMLFilesInCalls(t *testing.T) {
	tmpDir := t.TempDir()

	writeTestFile(t, filepath.Join(tmpDir, "Phones.vcf"), "BEGIN:VCARD\nVERSION:3.0\nFN:Owner\nitem1.TEL:+12025550100\nitem1.X-ABLabel:Google Voice\nTEL;TYPE=CELL:+12025550199\nEND:VCARD\n")
	writeTestFile(t, filepath.Join(tmpDir, "Calls", "+12025550123 - Text - 2026-04-27T00_00_00Z.html"), `<html><body>
<div class="participants">
  <cite class="sender"><a class="tel" href="tel:+12025550100"><span class="fn">Owner</span></a></cite>
  <cite class="sender"><a class="tel" href="tel:+12025550123"><span class="fn">Alice</span></a></cite>
</div>
<div class="hChatLog">
  <div class="message">
    <abbr class="dt" title="2026-04-27T00:00:00Z"></abbr>
    <cite class="sender"><a class="tel" href="tel:+12025550123"><span class="fn">Alice</span></a></cite>
    <q>Hello<img src="attachment"></q>
  </div>
</div>
</body></html>`)
	writeTestFile(t, filepath.Join(tmpDir, "Calls", "attachment.jpg"), "not really a jpg, just a sibling attachment placeholder")

	logs, observed := observer.New(zap.ErrorLevel)
	pipeline := make(chan *timeline.Graph, 1)

	err := new(FileImporter).FileImport(context.Background(), timeline.DirEntry{
		DirEntry: testDirEntry{name: ".", isDir: true},
		FS:       os.DirFS(tmpDir),
		Filename: ".",
	}, timeline.ImportParams{
		Pipeline: pipeline,
		Log:      zap.New(logs),
	})
	if err != nil {
		t.Fatalf("FileImport() error = %v", err)
	}
	close(pipeline)

	if observed.Len() != 0 {
		t.Fatalf("expected no error logs, got %d: %+v", observed.Len(), observed.All())
	}

	graph, ok := <-pipeline
	if !ok {
		t.Fatal("expected one imported graph")
	}
	if graph == nil || graph.Item == nil {
		t.Fatal("expected imported graph item")
	}
	if got := graph.Item.Owner.Name; got != "Alice" {
		t.Fatalf("expected imported message owner Alice, got %q", got)
	}

	var attachment *timeline.Item
	for _, edge := range graph.Edges {
		if edge.Relation == timeline.RelAttachment {
			attachment = edge.To.Item
			break
		}
	}
	if attachment == nil {
		t.Fatal("expected imported attachment edge")
	}
	if attachment.Content.Filename != "attachment.jpg" {
		t.Fatalf("expected attachment filename attachment.jpg, got %q", attachment.Content.Filename)
	}

	r, err := attachment.Content.Data(context.Background())
	if err != nil {
		t.Fatalf("attachment content data error = %v", err)
	}
	defer r.Close()

	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading attachment content = %v", err)
	}
	if string(body) != "not really a jpg, just a sibling attachment placeholder" {
		t.Fatalf("unexpected attachment content %q", string(body))
	}
}

type testDirEntry struct {
	name  string
	isDir bool
}

func (t testDirEntry) Name() string               { return t.name }
func (t testDirEntry) IsDir() bool                { return t.isDir }
func (t testDirEntry) Type() os.FileMode          { return 0 }
func (t testDirEntry) Info() (os.FileInfo, error) { return nil, nil }

func writeTestFile(t *testing.T, filename, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatalf("creating %s: %v", filepath.Dir(filename), err)
	}
	if err := os.WriteFile(filename, []byte(contents), 0o644); err != nil {
		t.Fatalf("writing %s: %v", filename, err)
	}
}
