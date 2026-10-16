package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

func testApp(t *testing.T) *App { return &App{configDir: t.TempDir()} }

func TestPreviewListsNamesToFix(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, norm.NFD.String("보고서.txt")), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "ok.txt"), []byte("y"), 0o644)
	plan, err := testApp(t).Preview(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].To != "보고서.txt" || plan[0].Status != "pending" {
		t.Fatalf("preview = %+v", plan)
	}
}

func csvFiles(root string) []string {
	entries, _ := os.ReadDir(root)
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "수정목록_") && strings.HasSuffix(e.Name(), ".csv") {
			out = append(out, e.Name())
		}
	}
	return out
}

// 수정 목록 CSV는 체크했을 때만 만든다(2026-10-16 사용자 피드백)
func TestFixAllWritesListCSVOnlyWhenAsked(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, norm.NFD.String("회의록.txt")), []byte("x"), 0o644)
	res, err := testApp(t).FixAll(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].Status != "done" || res.CSV != "" || len(csvFiles(root)) != 0 {
		t.Fatalf("체크하지 않으면 CSV가 없어야 한다: %+v %v", res, csvFiles(root))
	}

	os.WriteFile(filepath.Join(root, norm.NFD.String("사진.txt")), []byte("x"), 0o644)
	res, err = testApp(t).FixAll(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.CSV == "" || filepath.Dir(res.CSV) != root || len(csvFiles(root)) != 1 {
		t.Fatalf("체크하면 수정목록_날짜.csv를 그 폴더에 남겨야 한다: %+v %v", res, csvFiles(root))
	}
}

func TestFixFilesRenamesOnlyChosenFiles(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, norm.NFD.String("보고서.txt"))
	b := filepath.Join(root, norm.NFD.String("남김.txt"))
	os.WriteFile(a, []byte("x"), 0o644)
	os.WriteFile(b, []byte("y"), 0o644)
	app := testApp(t)
	plan, err := app.PreviewPaths([]string{a})
	if err != nil || len(plan) != 1 {
		t.Fatalf("preview files = %+v, %v", plan, err)
	}
	res, err := app.FixPaths([]string{a}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].Status != "done" || filepath.Dir(res.CSV) != root {
		t.Fatalf("fix files = %+v", res)
	}
	if _, err := os.Stat(b); err != nil {
		t.Fatal("고르지 않은 파일까지 바뀌었다")
	}
}

func TestFixRejectsEmptyChoice(t *testing.T) {
	if _, err := testApp(t).FixAll("", true); err == nil {
		t.Fatal("폴더를 고르지 않았으면 오류여야 한다")
	}
	if _, err := testApp(t).FixPaths(nil, true); err == nil {
		t.Fatal("파일을 고르지 않았으면 오류여야 한다")
	}
}

// 여러 폴더에서 끌어다 놓아도 수정 목록은 첫 번째 항목이 있는 폴더에 남긴다(2026-10-16 사용자 피드백)
func TestFixPathsWritesCSVNextToFirstItem(t *testing.T) {
	one, two := t.TempDir(), t.TempDir()
	a := filepath.Join(two, norm.NFD.String("가.txt"))
	b := filepath.Join(one, norm.NFD.String("나.txt"))
	os.WriteFile(a, []byte("x"), 0o644)
	os.WriteFile(b, []byte("y"), 0o644)
	res, err := testApp(t).FixPaths([]string{a, b}, true)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(res.CSV) != two || len(csvFiles(one)) != 0 {
		t.Fatalf("CSV = %q, want in %q", res.CSV, two)
	}
}

// 체크박스 상태는 프로그램을 닫았다 열어도 남는다
func TestSettingsPersist(t *testing.T) {
	dir := t.TempDir()
	if s := (&App{configDir: dir}).GetSettings(); s.MakeCSV {
		t.Fatal("처음에는 CSV를 만들지 않는다")
	}
	if err := (&App{configDir: dir}).SaveSettings(Settings{MakeCSV: true}); err != nil {
		t.Fatal(err)
	}
	if s := (&App{configDir: dir}).GetSettings(); !s.MakeCSV {
		t.Fatal("다시 열었을 때 체크 상태가 남아야 한다")
	}
}

// 의견 보내기에 함께 붙는 정보: 앱 버전(빌드 때 -ldflags로 넣음, 기본 dev)과 운영체제
func TestGetInfo(t *testing.T) {
	info := testApp(t).GetInfo()
	if info.Version != "dev" || info.OS == "" {
		t.Fatalf("info = %+v", info)
	}
}
