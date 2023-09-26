package common

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/text/encoding/charmap"
	"gopkg.in/ini.v1"
)

//MillimeterOfMercuryToKiloPascal мм рт. ст. в кПа
func MillimeterOfMercuryToKiloPascal(mmm float64) float64 {
	return mmm * 0.13332
}

//AtToKiloPascal Технические атмосферы (кгс/см²) в  кПа
func AtToKiloPascal(pressAt float64) float64 {
	return math.RoundToEven(pressAt * 98.067)
}

//KiloPascalToAt кПа в технические атмосферы (кгс/см²)
func KiloPascalToAt(pressKpa float64) float64 {
	return pressKpa * 0.01019716212978
}

//Decode1251String Декодирует строку из кодировки Windows1251 в обычную (UTF-8).
func Decode1251String(sNonUnicode string) string {
	r, _ := charmap.Windows1251.NewDecoder().String(sNonUnicode)
	return r
}

//Make1251String Делает строку в кодировке Windows1251 из обычной (UTF-8).
func Make1251String(sUTF8str string) string {
	r, _ := charmap.Windows1251.NewEncoder().String(sUTF8str)
	return r
}

//NullTerminatedToString Конвертация null-terminated string в строку
func NullTerminatedToString(cstr []byte) (result string) {
	if nil == cstr {
		return
	}
	if 0 == len(cstr) {
		return
	}

	indexNull := -1
	for i, val := range cstr {
		if val == 0x00 {
			indexNull = i
			break
		}
	}
	if indexNull >= 0 {
		result = string(cstr[:indexNull])
	}

	return
}

// RadianToDegree перевести координаты к градусам
// rad - рад-8 !!! Координаты передаются в -8 степени, чтобы избежать передачи float по CAN и clMGP
func RadianToDegree(rad uint32) (degree, min, sec float64) {

	// тут в общем перевести радианы в градусы, целая часть это градусы,
	// дробная часть * 60 это мин, от мин взять дробную часть * 60 это все остальное

	//rad := uint32(49701942) // longitude or latitude
	fval := ((float64(rad) / 100000000) * 180) / math.Pi
	i, f := math.Modf(fval) // integer and fractional
	degree = i
	min = math.Trunc(f * 60) // (отбросить дробную)
	_, f = math.Modf(f * 60)
	sec = f * 60
	// fmt.Printf("%.0f°%.0f´%2.2f´´\n", degree, min, sec) //28°28´37.61´´

	return
}

// RpmCalculation рассчитать значения частоты вращения вала дизель-генераторной установки от
// SpeedKmH -- скорости км\ч,
// iNumberTeeth -- числа зубов ДУП,
// iNumberOfPulses -- числа импульсов на один оборот вала
// iBandageDiameter -- диаметра колесной пары
// на ЖД бандаж может быть разным, но скрипты не учитывают этого
// ! все скрипты подразумевают одинаковый бандаж для первой и второй колесной пары !
func RpmCalculation(SpeedKmH, iBandageDiameter, iNumberTeeth, iNumberOfPulses int) uint32 {
	var SpeedMS float64                // скорость м/с
	var CircleL float64                // длинна окружности
	var Diameter float64               // диаметр бандажа в м
	var NumberSec float64              // обороты в сек
	var ImpulseSec, ImpulseMin float64 // имп./сек, имп./мин
	var Rpm float64                    // искомая частота об./мин

	SpeedMS = float64(SpeedKmH) / 3.6              // = 2.77 м/с для 10 км\ч
	Diameter = float64(iBandageDiameter) / 1000    // = 1.350 м
	CircleL = Diameter * math.Pi                   // = 4.24 м
	NumberSec = SpeedMS / CircleL                  // = 0.655 об/с
	ImpulseSec = NumberSec * float64(iNumberTeeth) // = 27.5 имп./сек (iNumberTeeth УПП - 42)
	ImpulseMin = ImpulseSec * 60                   // = 1650.496
	Rpm = ImpulseMin / float64(iNumberOfPulses)    // = 33 (iNumberOfPulses УПП - 50)

	return uint32(Rpm + 0.5)
}

// FloatRound использовать при получении float (c IPK или по CAN)
// pos -- количество знаков после запятой,  0 -- до целого
func FloatRound(fval float64, pos uint) (result float64) {
	negative := 1
	if fval < 0 {
		negative = -1
	}

	ftemp := math.Abs(fval)*math.Pow(10, float64(pos)) + 0.5
	itemp := int64(ftemp)
	result = float64(itemp) / math.Pow(10, float64(pos)) * float64(negative)

	return
}

//--------------------------------------------------------------------------------//

// GetOwnNameFolder получить имя папки из текущего пути  (GetProgramFiles -- не подходит тк МС устанавливают в разные каталоги, а не только в Wizard)
// num: 0-текущий каталог, 1-на каталог выше, 2-на 2 каталога выше итд
func GetOwnNameFolder(num int) (dir string) {
	dir, _ = os.Getwd()

	for num > 0 {
		dir = filepath.Dir(dir) // путь на каталог выше запущенного файла
		num--
	}

	return dir
}

// NeedLog смотрим в файле Wizard\settings.ini разрешена ли запись в лог
// смотрим раздел Debug строка log=1
func NeedLog() (enable bool, err error) {

	dir := GetOwnNameFolder(1) // путь до основной папки (wizard)

	iniPath := dir + `\settings.ini`
	cfg, err := ini.Load(iniPath)
	if err != nil {
		ShowErrorMessage("Не найден файл settings.ini", "")
		fmt.Println(err)
		return
	}

	PROP := "Debug"
	if v, err := cfg.Section(PROP).Key("log").Int(); err == nil {
		if v == 1 {
			enable = true
		}
	}
	return
}

// CreateLogFile создать папку для файлов Wizard\log, если не создана
// создать файл с именем проверки + временем
func CreateLogFile(nameCheck string) (file *os.File, err error) {

	dir := GetOwnNameFolder(1) + `\log`

	err = os.Mkdir(dir, 0777)
	if err != nil {
		fmt.Println(err)
	}

	t := time.Now()
	lgNameFile := dir + `\` + nameCheck + ` ` + t.Format("2006.01.02 15.04.05") + ".log"

	file, err = os.OpenFile(lgNameFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Ошибка создания log файла, error: %v\n", err)
	}

	return
}

//--------------------------------------------------------------------------------//
