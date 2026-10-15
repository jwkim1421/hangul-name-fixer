package core

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/text/unicode/norm"
)

type tc struct {
	Name, Input, Expected string
	Changed               bool
}

func loadCases(t *testing.T) []tc {
	raw, err := os.ReadFile("../testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct{ Cases []tc }
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Cases
}

// 소스 파일에 NFD 글자를 직접 적으면 편집기가 NFC로 바꿀 수 있어 코드로 만든다
func nfd(s string) string { return norm.NFD.String(s) }

func TestFixName(t *testing.T) {
	cases := loadCases(t)
	if len(cases) == 0 {
		t.Fatal("사례가 없다")
	}
	for _, c := range cases {
		got, changed := FixName(c.Input)
		if got != c.Expected || changed != c.Changed {
			t.Errorf("%s: got %q,%v want %q,%v", c.Name, got, changed, c.Expected, c.Changed)
		}
	}
}

func TestPlanAndApply(t *testing.T) {
	root := t.TempDir()
	nfdDir := filepath.Join(root, nfd("사진"))
	os.MkdirAll(nfdDir, 0o755)
	os.WriteFile(filepath.Join(nfdDir, nfd("보고서.txt")), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "보고서.txt"), []byte("y"), 0o644) // 이미 정상(NFC)

	plan, err := Plan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || plan[0].To != "보고서.txt" || plan[1].To != "사진" {
		t.Fatalf("plan = %+v (자식 파일 먼저, 그다음 폴더)", plan)
	}
	undo := filepath.Join(root, "되돌리기.csv")
	res := Apply(plan, undo)
	for _, r := range res {
		if r.Status != "done" {
			t.Fatalf("apply: %+v", r)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "사진", "보고서.txt")); err != nil {
		t.Fatal("고친 경로가 없다:", err)
	}
	if _, err := os.Stat(undo); err != nil {
		t.Fatal("되돌리기.csv가 없다")
	}
}

func TestExistingTargetIsSkipped(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, nfd("보고서.txt")), []byte("nfd"), 0o644)
	if err := os.WriteFile(filepath.Join(root, "보고서.txt"), []byte("nfc"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) < 2 && runtime.GOOS == "darwin" {
		t.Skip("APFS는 NFD/NFC 이름을 같은 파일로 본다 — 윈도우 CI에서 확인")
	}
	plan, _ := Plan(root)
	res := Apply(plan, "")
	if len(res) != 1 || res[0].Status != "skipped-exists" {
		t.Fatalf("같은 이름이 있으면 건너뛰어야 한다: %+v", res)
	}
}

// 되돌리기 CSV의 "현재 경로"는 부모 폴더까지 고친 뒤에도 실제로 있어야 한다
func TestUndoPathsExistAfterParentRenamed(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, nfd("사진"))
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, nfd("보고서.txt")), []byte("x"), 0o644)
	plan, _ := Plan(root)
	undo := filepath.Join(root, "undo.csv")
	Apply(plan, undo)
	f, err := os.Open(undo)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	f.Seek(3, 0) // BOM
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if _, err := os.Stat(row[0]); err != nil {
			t.Errorf("되돌리기 CSV의 경로가 없다: %q", row[0])
		}
		if norm.NFC.String(row[0]) != row[0] {
			t.Errorf("현재 경로에 풀어진 글자가 남았다: %q", row[0])
		}
	}
}
