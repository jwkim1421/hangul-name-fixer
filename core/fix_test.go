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
	undo := filepath.Join(root, "수정목록.csv")
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
		t.Fatal("수정목록.csv가 없다")
	}
}

// 같은 이름이 이미 있으면 덮어쓰지 않고 접미사 (1)을 붙인다(2026-10-16 사용자 결정 — 예전에는 건너뛰었다)
func TestExistingTargetGetsSuffix(t *testing.T) {
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
	if len(res) != 1 || res[0].Status != "done-suffix" || res[0].To != "보고서 (1).txt" {
		t.Fatalf("같은 이름이 있으면 (1)을 붙여야 한다: %+v", res)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "보고서.txt")); string(b) != "nfc" {
		t.Fatal("원래 있던 파일이 덮어써졌다")
	}
}

func TestUniqueName(t *testing.T) {
	taken := map[string]bool{"보고서.txt": true, "보고서 (1).txt": true, "회의록 (1).txt": true, "사진": true, "메모.tar.gz": true, ".env": true}
	is := func(n string) bool { return taken[n] }
	cases := []struct {
		name  string
		isDir bool
		want  string
	}{
		{"보고서.txt", false, "보고서 (2).txt"},     // (1)도 있으면 (2)
		{"회의록 (1).txt", false, "회의록 (2).txt"}, // (1)(1)이 아니라 (2)
		{"사진", true, "사진 (1)"},                // 폴더는 확장자 없음
		{"메모.tar.gz", false, "메모.tar (1).gz"},
		{".env", false, ".env (1)"},     // 점으로 시작하는 이름은 확장자가 아니다
		{"새 이름.txt", false, "새 이름.txt"}, // 겹치지 않으면 그대로
	}
	for _, c := range cases {
		if got := UniqueName(c.name, c.isDir, is); got != c.want {
			t.Errorf("UniqueName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// 파일 선택·끌어다 놓기: 고른 파일은 그 이름만, 고른 폴더는 안쪽(자식 먼저)과 폴더 자신의 이름까지.
// 같은 것을 두 번 넣어도 한 번만.
func TestPlanPaths(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, nfd("보고서.txt"))
	b := filepath.Join(root, "ok.txt")
	dir := filepath.Join(root, nfd("사진"))
	os.WriteFile(a, []byte("x"), 0o644)
	os.WriteFile(b, []byte("y"), 0o644)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, nfd("여행.jpg")), []byte("z"), 0o644)
	plan := PlanPaths([]string{a, b, a, dir})
	if len(plan) != 3 || plan[0].To != "여행.jpg" || plan[0].Dir != dir {
		t.Fatalf("폴더 안쪽이 먼저여야 한다: %+v", plan)
	}
	got := map[string]bool{}
	for _, r := range plan {
		got[r.Dir+"|"+r.To] = r.Status == "pending"
	}
	if !got[root+"|보고서.txt"] || !got[root+"|사진"] {
		t.Fatalf("고른 파일과 폴더 자신의 이름도 고쳐야 한다: %+v", plan)
	}
	for _, r := range Apply(plan, "") {
		if r.Status != "done" {
			t.Fatalf("apply: %+v", r)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "사진", "여행.jpg")); err != nil {
		t.Fatal("고친 경로가 없다:", err)
	}
}

// 수정 목록 CSV의 "수정된 경로"는 부모 폴더까지 고친 뒤에도 실제로 있어야 한다
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
			t.Errorf("수정 목록 CSV의 경로가 없다: %q", row[0])
		}
		if norm.NFC.String(row[0]) != row[0] {
			t.Errorf("현재 경로에 풀어진 글자가 남았다: %q", row[0])
		}
	}
}
