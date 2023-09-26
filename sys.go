package common

import (
	"os"

	"github.com/kbinani/win"
)

//Is64bitWindows определяет разрядность Windows.
//true - 64 бит
//false - 32 бит
func Is64bitWindows() (is64bit bool) {
	is64bit = true
	var wowDir [260]uint16
	if 0 == win.GetSystemWow64Directory(&wowDir[0], 260) {
		is64bit = false
	}
	return
}

//GetProgramFiles возвращает путь до системной папки Program Files
func GetProgramFiles() (programFiles string) {
	if !Is64bitWindows() {
		programFiles = os.Getenv("programfiles")
	} else {
		programFiles = os.Getenv("programfiles(x86)")
	}
	return programFiles
}
