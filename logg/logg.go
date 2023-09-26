package logg

import (
	"log"
	"os"

	"github.com/artlukm/common"
)

// DataLOG data
type DataLOG struct {
	enabled bool
	// fileName string
	file    *os.File
	logFile *log.Logger
}

// EnableLOG включить записть в лог
func (d *DataLOG) EnableLOG(ok bool) {
	d.enabled = ok
}

// InitLOG создание файла (с названием "Check nameCheck 2006.01.02 15.04.05.log") и запись в него заголовка
// nameCheck имя проверки, например "Поверка БУ-4"
func (d *DataLOG) InitLOG(nameCheck string) (err error) {

	f, err := common.CreateLogFile(nameCheck)

	// заголовок
	l := log.New(f, "FILE: ", log.Ldate|log.Ltime)
	l.Printf(nameCheck + "\r\n")

	// дальше добавлять только время
	l.SetFlags(log.Ltime | log.Lmicroseconds) //log.Lshortfile -- нет смысла выводить имя и строку файла, он всегда покажет одно и тоже -- пакет logg

	d.file = f
	d.logFile = l

	// файл создается только при тестах и отладки (чтобы не забивать комп лишними файлами)
	// если был создан файл, то значит записываем в него данные, если файл не создан то все строчки
	// lg.Print и lg.Error  в скрипте остаются но ничего не записывают, ошибки не вызывают
	d.enabled = true

	return
}

// CloseFile close
func (d *DataLOG) CloseFile() {
	d.file.Close()
}

// Title new line + msg
func (d *DataLOG) Title(msg string) {
	if !d.enabled || d == nil {
		return
	}
	d.logFile.SetPrefix("\nTITLE: \t")
	d.logFile.Println(msg)
}

// Print msg
func (d *DataLOG) Print(msg string) {
	if !d.enabled || d == nil {
		return
	}
	d.logFile.SetPrefix("INFO: \t")
	d.logFile.Println(msg)
}

// Error print error
func (d *DataLOG) Error(msg string) {
	if !d.enabled || d == nil {
		return
	}
	d.logFile.SetPrefix("ERROR: \t")
	d.logFile.Println(msg)
}

/*
var lg logg.DataLOG

func main() {

	if ok, err := common.NeedLog(); ok && err == nil { // определить нужен ли файл по settings.ini
		lg.InitLOG("Проверка БУ") // если файл log не нужен не вызывать эту строчку
		defer lg.CloseFile()
	}
	lg.Print("Тут такой типа строк 1")
	lg.Error("Ошибка 1")
*/
