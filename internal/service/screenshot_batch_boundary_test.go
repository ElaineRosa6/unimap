package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScreenshotBatchOutsideLink(t *testing.T) {
	for _, relative := range []bool{false, true} {
		name := "absolute"
		if relative {
			name = "relative"
		}
		t.Run(name, func(t *testing.T) {
			for _, action := range []string{"list", "delete-file", "delete-batch"} {
				t.Run(action, func(t *testing.T) {
					fixture := t.TempDir()
					base, outside := filepath.Join(fixture, "screenshots"), filepath.Join(fixture, "outside")
					for _, dir := range []string{base, outside} {
						if err := os.Mkdir(dir, 0700); err != nil {
							t.Fatal(err)
						}
					}
					victim := filepath.Join(outside, "keep.png")
					if err := os.WriteFile(victim, []byte("preserve fixture"), 0600); err != nil {
						t.Fatal(err)
					}
					target := outside
					if relative {
						target = filepath.Join("..", "outside")
					}
					if err := os.Symlink(target, filepath.Join(base, "batch")); err != nil {
						t.Fatalf("create fixture symlink: %v", err)
					}
					svc := NewScreenshotAppService(base)
					switch action {
					case "list":
						called := false
						files, err := svc.ListBatchFiles("batch", func(string) string { called = true; return "preview" })
						if err == nil || len(files) != 0 || called {
							t.Errorf("outside listing: files=%v err=%v preview=%v", files, err, called)
						}
					case "delete-file":
						if err := svc.DeleteFile("batch", "keep.png"); err == nil {
							t.Error("outside file deletion accepted")
						}
					case "delete-batch":
						// Removing only the link is acceptable; following its target is not.
						_ = svc.DeleteBatch("batch")
					}
					data, err := os.ReadFile(victim)
					if err != nil || string(data) != "preserve fixture" {
						t.Errorf("outside fixture changed: %q, %v", data, err)
					}
				})
			}
		})
	}
}

func TestScreenshotBatchNestedLinkRemoval(t *testing.T) {
	fixture := t.TempDir()
	base, outside := filepath.Join(fixture, "screenshots"), filepath.Join(fixture, "outside")
	for _, dir := range []string{filepath.Join(base, "batch"), outside} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	victim := filepath.Join(outside, "keep.png")
	if err := os.WriteFile(victim, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, "batch", "link")); err != nil {
		t.Fatal(err)
	}
	if err := NewScreenshotAppService(base).DeleteBatch("batch"); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(victim); err != nil || string(data) != "keep" {
		t.Fatalf("outside fixture changed: %q, %v", data, err)
	}
}

func TestScreenshotBatchRegularFilesOnly(t *testing.T) {
	base := t.TempDir()
	batch := filepath.Join(base, "batch")
	if err := os.Mkdir(batch, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(batch, "real.png"), []byte("png"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.png", filepath.Join(batch, "link.png")); err != nil {
		t.Fatal(err)
	}
	svc := NewScreenshotAppService(base)
	previews := 0
	files, err := svc.ListBatchFiles("batch", func(string) string { previews++; return "preview" })
	if err != nil || len(files) != 1 || previews != 1 {
		t.Fatalf("regular file listing = %v, %v; previews=%d", files, err, previews)
	}
	if files[0].Name != "real.png" {
		t.Fatalf("unexpected file %q", files[0].Name)
	}
	batches, err := svc.ListBatches()
	if err != nil || len(batches) != 1 || batches[0].FileCount != 1 {
		t.Fatalf("batch count = %v, %v", batches, err)
	}
	if err := svc.DeleteFile("batch", "real.png"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteBatch("batch"); err != nil {
		t.Fatal(err)
	}
}

func TestScreenshotBatchMissingClassification(t *testing.T) {
	base := t.TempDir()
	for _, dir := range []string{base, filepath.Join(base, "absent-root")} {
		svc := NewScreenshotAppService(dir)
		_, listErr := svc.ListBatchFiles("missing", nil)
		for _, err := range []error{listErr, svc.DeleteBatch("missing"), svc.DeleteFile("missing", "file.png")} {
			if err == nil || !strings.Contains(err.Error(), "not found") {
				t.Errorf("missing-path classification: %v", err)
			}
		}
	}
}
