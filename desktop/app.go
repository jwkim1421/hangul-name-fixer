// 윈도우 프로그램(Wails): 화면은 브라우저 버전(web/)을 그대로 쓰고, 이름 바꾸기는 Go core가 한다.
package main

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/jwkim1421/hangul-name-fixer/core"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App은 화면(JavaScript)에서 window.go.main.App.* 로 부르는 함수들을 가진다.
type App struct{ ctx context.Context }

func NewApp() *App                         { return &App{} }
func (a *App) startup(ctx context.Context) { a.ctx = ctx }

var errNoFolder = errors.New("고칠 폴더를 먼저 고르세요")

// ChooseFolder는 폴더 선택 창을 띄운다. 취소하면 "".
func (a *App) ChooseFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "고칠 폴더를 고르세요"})
}

// Preview는 바꿀 이름 목록(깊은 곳부터)을 돌려준다. 아무것도 바꾸지 않는다.
func (a *App) Preview(root string) ([]core.Rename, error) {
	if root == "" {
		return nil, errNoFolder
	}
	return core.Plan(root)
}

// FixAll은 이름을 고치고 root\되돌리기_YYYYMMDD_HHMMSS.csv를 남긴다.
func (a *App) FixAll(root string) ([]core.Rename, error) {
	if root == "" {
		return nil, errNoFolder
	}
	plan, err := core.Plan(root)
	if err != nil {
		return nil, err
	}
	undo := filepath.Join(root, "되돌리기_"+time.Now().Format("20060102_150405")+".csv")
	return core.Apply(plan, undo), nil
}
