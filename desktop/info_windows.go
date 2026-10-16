package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// 윈도우 11도 내부 버전은 10.0이라 빌드 번호(22000 이상이 11)까지 적는다
func osVersion() string {
	v := windows.RtlGetVersion()
	return fmt.Sprintf("Windows %d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
}
