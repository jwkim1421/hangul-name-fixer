package main

import "runtime"

// version은 빌드할 때 -ldflags "-X main.version=v0.1.0"으로 넣는다(release.yml). 직접 빌드하면 dev.
var version = "dev"

// Info는 의견 보내기에 함께 붙는 정보다(파일 이름·경로는 넣지 않는다).
type Info struct {
	Version string
	OS      string // 예: "Windows 10.0.22631 (amd64)"
}

// GetInfo는 앱 버전과 운영체제를 돌려준다.
func (a *App) GetInfo() Info {
	return Info{Version: version, OS: osVersion() + " (" + runtime.GOARCH + ")"}
}
