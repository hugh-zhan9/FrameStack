package devsample_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"idea/internal/devsample"
)

func TestGenerateCreatesExpectedSampleFiles(t *testing.T) {
	root := t.TempDir()

	report, err := devsample.Generate(root)
	if err != nil {
		t.Fatalf("expected generator to succeed: %v", err)
	}
	if report.FileCount != 5 {
		t.Fatalf("expected 5 files, got %#v", report)
	}
	if report.DuplicatePair[0] == "" || report.DuplicatePair[1] == "" {
		t.Fatalf("expected duplicate pair paths, got %#v", report)
	}

	for _, rel := range []string{
		"photos/set-a/model-a-001.png",
		"photos/set-a/model-a-001-copy.png",
		"photos/set-a/model-a-002.png",
		"photos/set-b/model-b-001.png",
		"images/posters/poster-sample.png",
	} {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", rel, err)
		}
	}
}

func TestGenerateProducesByteIdenticalDuplicatePair(t *testing.T) {
	root := t.TempDir()

	report, err := devsample.Generate(root)
	if err != nil {
		t.Fatalf("expected generator to succeed: %v", err)
	}

	left, err := os.ReadFile(report.DuplicatePair[0])
	if err != nil {
		t.Fatalf("read duplicate left: %v", err)
	}
	right, err := os.ReadFile(report.DuplicatePair[1])
	if err != nil {
		t.Fatalf("read duplicate right: %v", err)
	}
	if !bytes.Equal(left, right) {
		t.Fatalf("expected duplicate pair to be byte identical")
	}
}
