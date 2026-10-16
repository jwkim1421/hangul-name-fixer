package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

func TestPreviewListsNamesToFix(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, norm.NFD.String("보고서.txt")), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "ok.txt"), []byte("y"), 0o644)
	plan, err := NewApp().Preview(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].To != "보고서.txt" || plan[0].Status != "pending" {
		t.Fatalf("preview = %+v", plan)
	}
}

func TestFixAllRenamesAndWritesTimestampedUndoCSV(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, norm.NFD.String("회의록.txt")), []byte("x"), 0o644)
	res, err := NewApp().FixAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Status != "done" {
		t.Fatalf("fixall = %+v", res)
	}
	entries, _ := os.ReadDir(root)
	var undo string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "되돌리기_") && strings.HasSuffix(e.Name(), ".csv") {
			undo = e.Name()
		}
	}
	if undo == "" {
		t.Fatalf("되돌리기_YYYYMMDD_HHMMSS.csv가 없다: %v", entries)
	}
}

func TestFixAllRejectsEmptyRoot(t *testing.T) {
	if _, err := NewApp().FixAll(""); err == nil {
		t.Fatal("폴더를 고르지 않았으면 오류여야 한다")
	}
}
