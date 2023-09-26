package common

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

///Функции для перевода сообщений сценариев на другие языки:

//Translation структура, содержащая переводы строк. Файл перевода состоит из массива таких структур, упакованных в json.
type Translation struct {
	LangTag string            //Тэг языка
	Strings map[string]string //Строки для перевода. Ключ - строка на русском; значение - это перевод.
}

//Lang тип для получения переведённых строк
type Lang struct {
	Tag string

	langlist          []string
	strings           []Translation
	WarnNoTranslation bool // если установить true, то будут выводиться предупреждения при обращении к строкам, для которых нет переводв
}

//Str возвращает перевод строки str или саму строку в неизменном виде если перевода нет
func (lang *Lang) Str(str string) string {
	if nil == lang {
		return str
	}

	for _, trans := range lang.strings {
		if trans.LangTag == lang.Tag {
			_, exists := trans.Strings[str]
			if exists {
				return trans.Strings[str]
			}
			if lang.WarnNoTranslation {
				fmt.Fprintf(os.Stderr, "\"%s\" - no %s translation!\r\n", str, lang.Tag)
			}
		}
	}

	return str
}

//Init загружает файл с переводами.
//filename - имя файла со строками переводов.
func (lang *Lang) Init(filename string) (err error) {
	if nil == lang {
		err = fmt.Errorf("%s", "init error")
		return
	}

	lang.strings, err = lang.loadStrings(filename)
	if err != nil {
		return
	}
	for _, trans := range lang.strings {
		lang.langlist = append(lang.langlist, trans.LangTag) // добавляем язык в список доступных языков
		//fmt.Printf("Load language: %s\r\n", languageTag.String())
	}
	return
}

//Activate устанавливает активный язык.
func (lang *Lang) Activate(language string) (err error) {
	if nil == lang {
		err = fmt.Errorf("%s", "nil")
		return
	}

	//Ищем запрашиваемый язык среди доступных
	langFound := false
	for _, langname := range lang.langlist {
		if language == langname {
			langFound = true
			break
		}
	}

	if !langFound {
		err = fmt.Errorf("Language %s is not available", language)
		return
	}

	lang.Tag = language

	return
}

/*
Загружает json-файл, который содержит массив из структур типа commmon.Translation.
*/
func (lang *Lang) loadStrings(filename string) (readStrings []Translation, err error) {
	if nil == lang {
		err = fmt.Errorf("%s", "nil")
		return
	}

	var file *os.File

	fileinfo, ferr := os.Stat(filename)
	if ferr != nil {
		err = ferr
		return
	}

	if 0 == fileinfo.Size() {
		err = fmt.Errorf("%s", "empty file")
		return
	}

	file, err = os.Open(filename)

	if err != nil {
		return
	}
	defer file.Close()

	// reader := bufio.NewReader(file)

	// readbytes := make([]byte, fileinfo.Size())
	// _, err = reader.Read(readbytes)
	// if err != nil {
	// 	if err != io.EOF {
	// 		return
	// 	}
	// }

	zr, err := zlib.NewReader(file)
	if err != nil {
		err = fmt.Errorf("%s:%s", "unpack init error", err.Error())
		return
	}
	defer zr.Close()

	var b bytes.Buffer
	_, errcopy := io.Copy(&b, zr)
	if errcopy != nil {
		err = fmt.Errorf("%s:%s", "unpack read error", errcopy.Error())
		return
	}

	err = json.Unmarshal(b.Bytes(), &readStrings)

	if err != nil {
		err = fmt.Errorf("%s:%s", "unmarshal error", errcopy.Error())
		return
	}

	return
}
