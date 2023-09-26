package common

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/TheTitanrain/w32"
)

//MBTOPMOST - сообщение MB_TOPMOST - поверх всех окон
const cMBTOPMOST = 0x00040000
const cMBSERVICENOTIFICATION = 0x200000
const cMBSETFOREGROUND = 0x00010000

//хэндл окна Мастера сценариев, чтобы показ MessageBox был привязан к основному окну
func getMasterWindowHandle() (windowHandle w32.HWND) {

	//Когда сценарицй запущен, в заголовке главного окна будет имя сценария
	scriptName := strings.TrimSuffix(filepath.Base(os.Args[0]), `.exe`)

	//NOTE: здесь жёстко прописан русский заголовок окна. Переделать, если ВДРУГ кому-то понадобится иноязычная версия ИПК-3
	masterWindowName := "Мастер сценариев - " + scriptName

	title, err := syscall.UTF16PtrFromString(masterWindowName)
	if nil == err {
		windowHandle = w32.FindWindowW(nil, title)
	}
	return
}

//ShowInfoMessage показывает диалоговое окно с текстом сообщения и кнопкой ОК.
//s - текст сообщения.
//title - заголовок сообщения.
func ShowInfoMessage(s, title string) {
	if title == "" {
		title = " "
	}
	w32.MessageBox(getMasterWindowHandle(), s, title, w32.MB_OK|w32.MB_ICONINFORMATION|cMBTOPMOST|cMBSETFOREGROUND)
}

//ShowErrorMessage показывает диалоговое окно с текстом сообщения об ошибке и кнопкой ОК.
//s - текст сообщения.
//title - заголовок сообщения.
func ShowErrorMessage(s, title string) {
	if title == "" {
		title = " "
	}
	w32.MessageBox(getMasterWindowHandle(), s, title, w32.MB_OK|w32.MB_ICONERROR|cMBTOPMOST|cMBSETFOREGROUND)
}

//ShowDialogYesNo показывает диалоговое окно с текстом и кнопками Да/Нет. Возвращает true если пользователь нажал "Да".
//s - текст сообщения.
//title - заголовок сообщения.
func ShowDialogYesNo(s, title string) bool {
	if title == "" {
		title = " "
	}
	r := w32.MessageBox(getMasterWindowHandle(), s, title, w32.MB_YESNO|w32.MB_ICONQUESTION|cMBTOPMOST|cMBSETFOREGROUND)
	return r == w32.IDYES
}
