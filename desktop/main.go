package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title: "자/모음 분리 복구기",
		// 시작 화면은 작게 띄우고, 결과 표가 나오면 화면(app.js)이 창을 키운다.
		// 이보다 작게 줄이면 버튼과 문구가 밀려나 최소 크기를 둔다(2026-10-16 사용자 피드백)
		Width:     720,
		Height:    380,
		MinWidth:  600,
		MinHeight: 360,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 247, G: 245, B: 242, A: 1},
		OnStartup:        app.startup,
		// 파일·폴더를 창에 끌어다 놓으면 전체 경로를 화면(app.js, runtime.OnFileDrop)에 넘긴다
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
