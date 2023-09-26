package common

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

/*
Для расшифровки двоичных файлов МПМЭ используется консольная утилита-расшифровщик, которая имеет следующее описание:

Текстовый serial-расшифровщик формата БУ-3П
parser.exe [-команда] [входной файл данных]
Список команд:
	-e	Список параметров
	-k	Трассировка по ключу
	-t	Трассировка по времени
	-p	Трассировка по отдельным параметрам
*/

//ParserMPME Обёртка над расшифровщиком формата МПМЭ
type ParserMPME struct {
	ParserPath string // путь до исполняемого файла парсера
}

func fileOk(name string) (ok bool) {
	st, err := os.Stat(name)
	if err != nil || st.IsDir() {
		return
	}
	ok = true
	return
}

func dirOk(name string) (ok bool) {
	st, err := os.Stat(name)
	if err != nil || !st.IsDir() {
		return
	}
	ok = true
	return
}

//NewParserMPME возвращает объект ParserMPME если существует файл указанный в path
//exePath - полный путь к консольной утилите-парсеру.
func NewParserMPME(exePath string) (p *ParserMPME, err error) {

	if !fileOk(exePath) {
		err = fmt.Errorf("file %s not ok", exePath)
		return
	}

	pars := ParserMPME{ParserPath: exePath}
	p = &pars
	return
}

//GetStrings извлекает строки из двоичного файла МПМЭ. Если не может - возвращает err.
func (p *ParserMPME) GetStrings(binFileName string) (result []string, err error) {
	if nil == p {
		err = fmt.Errorf("ParserMPME:GetStrings() null ptr")
		return
	}
	if !fileOk(p.ParserPath) {
		err = fmt.Errorf("setup ParserMPME.ParserPath first")
		return
	}
	cmd := exec.Command(p.ParserPath, "-t", binFileName)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	//cmd.Stderr = &stderr
	err = cmd.Run()

	if err != nil {
		err = fmt.Errorf("ParserMPME:cmd.Run() failed with %s", err.Error())
	}
	mpme := bufio.NewScanner(bytes.NewBuffer(stdout.Bytes()))
	for mpme.Scan() {
		result = append(result, Decode1251String(mpme.Text()))
	}
	return
}
