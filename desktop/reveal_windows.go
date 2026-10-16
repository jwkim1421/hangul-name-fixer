package main

import (
	"os/exec"
	"syscall"
)

// explorer는 /select,"경로"를 한 덩어리로 받아야 해서 명령줄을 직접 만든다
// (exec가 인자를 따옴표로 감싸면 공백 있는 경로에서 선택이 안 된다). explorer는 성공해도 1을 돌려줘 Start만 한다.
func reveal(path string) error {
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	return cmd.Start()
}
