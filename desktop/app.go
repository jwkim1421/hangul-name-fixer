// 윈도우 프로그램(Wails): 화면은 브라우저 버전(web/)을 그대로 쓰고, 이름 바꾸기는 Go core가 한다.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/jwkim1421/hangul-name-fixer/core"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App은 화면(JavaScript)에서 window.go.main.App.* 로 부르는 함수들을 가진다.
type App struct {
	ctx       context.Context
	configDir string // 설정 파일 위치(윈도우: %AppData%\hangul-name-fixer)
}

func NewApp() *App {
	dir, _ := os.UserConfigDir()
	return &App{configDir: filepath.Join(dir, "hangul-name-fixer")}
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// FixResult는 고친 결과와, 수정 목록 CSV를 남겼으면 그 경로다.
type FixResult struct {
	Items []core.Rename
	CSV   string
}

// Settings는 프로그램을 닫았다 열어도 남는 선택이다.
type Settings struct {
	MakeCSV bool // 수정 목록(*.csv) 파일 생성
}

var (
	errNoFolder = errors.New("수정할 폴더를 먼저 선택하세요")
	errNoFiles  = errors.New("수정할 파일을 먼저 선택하거나 끌어다 놓으세요")
)

// ChooseFolder는 폴더 선택 창을 띄운다. 취소하면 "".
// 윈도우 폴더 선택 창은 파일을 보여 주지 않아, 고른 뒤 목록에 나온다는 것을 제목에 적는다.
func (a *App) ChooseFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "수정할 폴더를 선택하세요 — 안의 파일은 선택한 뒤 목록에 표시됩니다"})
}

// ChooseFiles는 파일 선택 창(여러 개 선택 가능)을 띄운다. 취소하면 빈 목록.
func (a *App) ChooseFiles() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{Title: "수정할 파일을 선택하세요"})
}

// Preview는 바꿀 이름 목록(깊은 곳부터)을 돌려준다. 아무것도 바꾸지 않는다.
func (a *App) Preview(root string) ([]core.Rename, error) {
	if root == "" {
		return nil, errNoFolder
	}
	return core.Plan(root)
}

// PreviewPaths는 고르거나 끌어다 놓은 파일·폴더에서 바꿀 이름 목록을 돌려준다.
func (a *App) PreviewPaths(paths []string) ([]core.Rename, error) {
	if len(paths) == 0 {
		return nil, errNoFiles
	}
	return core.PlanPaths(paths), nil
}

// FixAll은 폴더 안의 이름을 고친다. makeCSV면 그 폴더에 수정목록_YYYYMMDD_HHMMSS.csv를 남긴다.
func (a *App) FixAll(root string, makeCSV bool) (FixResult, error) {
	if root == "" {
		return FixResult{}, errNoFolder
	}
	plan, err := core.Plan(root)
	if err != nil {
		return FixResult{}, err
	}
	return apply(plan, root, makeCSV), nil
}

// FixPaths는 고르거나 끌어다 놓은 파일·폴더의 이름을 고친다. 여러 폴더에서 왔어도 CSV는
// 첫 번째 항목이 있는 폴더에 남긴다(그 폴더는 이름이 바뀌지 않는다).
func (a *App) FixPaths(paths []string, makeCSV bool) (FixResult, error) {
	if len(paths) == 0 {
		return FixResult{}, errNoFiles
	}
	return apply(core.PlanPaths(paths), filepath.Dir(filepath.Clean(paths[0])), makeCSV), nil
}

func apply(plan []core.Rename, dir string, makeCSV bool) FixResult {
	csv := ""
	if makeCSV {
		csv = filepath.Join(dir, "수정목록_"+time.Now().Format("20060102_150405")+".csv")
	}
	items := core.Apply(plan, csv)
	if _, err := os.Stat(csv); csv != "" && err != nil {
		csv = "" // 바뀐 이름이 하나도 없으면 CSV를 만들지 않는다
	}
	return FixResult{Items: items, CSV: csv}
}

func (a *App) settingsPath() string { return filepath.Join(a.configDir, "settings.json") }

// GetSettings는 저장된 선택을 읽는다. 없거나 읽지 못하면 기본값(CSV 만들지 않음).
func (a *App) GetSettings() Settings {
	var s Settings
	if b, err := os.ReadFile(a.settingsPath()); err == nil {
		json.Unmarshal(b, &s)
	}
	return s
}

// SaveSettings는 선택을 저장한다.
func (a *App) SaveSettings(s Settings) error {
	if err := os.MkdirAll(a.configDir, 0o755); err != nil {
		return err
	}
	b, _ := json.Marshal(s)
	return os.WriteFile(a.settingsPath(), b, 0o644)
}

// RevealInFolder는 탐색기(맥: Finder)에서 path가 있는 폴더를 열고 그 파일을 선택해 둔다.
func (a *App) RevealInFolder(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return reveal(path)
}
