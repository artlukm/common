package parsebin

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

/*	-------------------- анализ данных -------------------- */

func ifEmptyBlock(block []byte) bool {

	if len(block) != sizeBlock {
		return false
	}

	// b := make([]byte, 1)
	// b[0] = 0xFF
	// emptyBuffer := bytes.Repeat(b, sizeBlock) // создать срез из 64 0xFF

	// if bytes.Equal(block, emptyBuffer) {
	// 	return true
	// }

	for _, b := range block {
		if b != 0xFF {
			return false
		}
	}

	return true
}

// найти номер и смещение нужной поездки
func findHeader(count int, file *os.File) (number int, offset int64) {
	var headerNumbers []int                // номера шапок по всему файлу
	hedearPositions := make(map[int]int64) // смещения шапок относительно начала файла

	if count < 1 {
		count = 1 //одна поездка должна быть
	}

	for {

		block, err := readNextBytes(file, sizeBlock)
		if err == io.EOF {
			fmt.Println("Конец файла")
			break
		}

		if block[0] == 0xDA && block[1] == 0xDB && block[2] == 0xDC {

			headSlice := append(block[:sizeHeadBlock])
			h := head{}
			buffer := bytes.NewBuffer(headSlice)
			err := binary.Read(buffer, binary.LittleEndian, &h)
			if err != nil {
				fmt.Printf("Ошибка binary.Read(): %v", err) // todo переписать все ошибки
			}

			n := int(h.TripNumber)
			headerNumbers = append(headerNumbers, n)

			p, _ := file.Seek(0, 1) // Get the current offset so we can seek back to it.
			hedearPositions[n] = p
		}
	}
	fmt.Println(len(headerNumbers))
	if len(headerNumbers) <= count {
		number = headerNumbers[0]
		offset = 0
	} else {
		number = headerNumbers[len(headerNumbers)-count]
		offset = hedearPositions[number] - sizeBlock // смещение берем -1 блок -- встав на эту позицию и прочитав следующий блок, попадем на шапку number
	}

	// file.Seek(0, 0) //go to the begging of the file

	return
}

func parseBlock(block []byte) (strDebug []string, tail int, err error) {
	i := 0
	lenBlock := len(block) - 4 // длинна стандартного блока -- 64 байта, длинна данных в этом блоке 60 байт,
	// 4 последних байта КС контрольая сумма блока, но, тк событие может начинаться в одном блоке, а заканчиваться
	// в дугом (в этом случае начало события плюсуется к следующему блоку), то каждый раз длинна разная

	// Шапка (Маркер файла DAh, DBh, DC)
	if block[0] == 0xDA && block[1] == 0xDB && block[2] == 0xDC {

		headSlice := append(block[:sizeHeadBlock])
		h := head{}
		buffer := bytes.NewBuffer(headSlice)
		err = binary.Read(buffer, binary.LittleEndian, &h)
		if err != nil {
			fmt.Printf("Ошибка binary.Read(): %v", err) // todo переписать все ошибки
		}

		// ШАПКА = (№: 1; ПРОБЕГ: 3366.000 км; ТИП/№ ЛОКОМОТИВА: 222/333; ПРИЗНАК: 1; АЛС: 10; ЭПТ: 0; УСТАВКИ: 40, 30, 10, 0; ВРЕМЯ: 03:35:32)
		s := fmt.Sprintf("ШАПКА = (№: %d; ПРОБЕГ: %d км; ТИП/№ ЛОКОМОТИВА: %d/%d; ПРИЗНАК: %d; АЛС: %d; ЭПТ: %d; УСТАВКИ: %d, %d, %d, %d; ВРЕМЯ: %d:%d:%d)",
			h.TripNumber, h.Milage, h.TypeLoco, h.NumberLoco, h.Cab, h.AlsCode, h.PresentEPT, h.SpeedY, h.SpeedRY, h.SpeedU1, h.SpeedU2, h.TripTime.Hour, h.TripTime.Min, h.TripTime.Sec)
		strDebug = append(strDebug, s)

		i = sizeHeadBlock // сместить индекс на следующий за шапкой элемент
	}

	// все что осталось это ИД и их данные
	lenIvent := 1 // длинна текущего события в байтах вместе с ИД
	for ; i < lenBlock; i = i + lenIvent {

		id := uint(block[i]) // ИД текущего события

		if id <= 0x3B { // 00h..3Bh относительная регистрация времени 1 байт
			lenIvent = 1
			str, _ := parseTime(append(block[i : i+lenIvent]))
			strDebug = append(strDebug, str)
			continue
		}

		// проверить ИД, определить длинну
		if _, present := mapIvent[id]; !present {
			strDebug = append(strDebug, fmt.Sprintf("!!!\tНеизвестный ИД: 0x%X", id))
			continue // не обрабатываем
		} else {

			lenIvent = mapIvent[id].len
			if lenIvent == 0xFF { // переменная длинна
				if i+1 < lenBlock { // значит за длинну сообщения отвечает следующий за ИД байт
					lenIvent = int(block[i+1])
					switch id {
					case 0xB0:
						// Сбои: Идентификатор события – B0h. Первый байт операндов – количество записанных кодов сбоев (2 байта каждый).
						lenIvent = 2*lenIvent + 2 // N (кол-во сбоев по 2 байта) = block[i+1] + байт ИД и длинны: 2*N + 2
					case 0xF8, 0xF9:
						// Доп. параметры: длина события без учета идентификатора
						lenIvent++
					default:
					}
				}
			}
		}

		if (i + lenIvent - 1) >= lenBlock { // проверить целиком ли событие размещено в блоке
			tail = i // несколько байт этого события размещено в следующем блоке, сохраняем
			break
		}

		str := ""

		if id <= 0x3B { // 00h..3Bh относительная регистрация времени 1 байт
			lenIvent = 1
			str, _ = parseTime(block[i : i+lenIvent]) //+
		}

		switch id {

		case 0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4A, 0xD8, 0xD9, 0xDA, 0xDB, 0xDC, 0xDD:
			_, str, _ = parseSpeed(block[i : i+lenIvent]) //+

		case 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57:
			str, _ = parsePress(block[i : i+lenIvent]) //+

		case 0x58, 0x59, 0x5A, 0x5B, 0x5C, 0x5D, 0x5E:
			uCrane395Slice = append(uCrane395Slice, id) //+
			str = "\t" + mapIvent[id].description

		case 0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67:
			str, _ = parseALS(block[i : i+lenIvent]) //+

		case 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7A, 0x7B, 0X7C,
			0x7D, 0x7E, 0x91, 0x92, 0xA5, 0xA6, 0xAB, 0xAC, 0xA9, 0xAA:
			mapBUSM[id] = mapIvent[id] //+
			str = "\t" + mapIvent[id].description

		case 0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8A:
			uPeriodSlice = append(uPeriodSlice, id) //+
			str = "\t" + mapIvent[id].description

		case 0xA0, 0xA1, 0xA2, 0xA3:
			uCodesRCSlice = append(uCodesRCSlice, id) //+
			str = "\t" + mapIvent[id].description

		case 0xAD:
			str, _ = parseFireAlarmSignals(block[i : i+lenIvent]) //+

		case 0xB0:
			str, _ = parseFaults(block[i : i+lenIvent]) //+

		case 0xD1, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7:
			str, _ = parseAnalogSignal(block[i : i+lenIvent]) //+

		case 0xC8:
			_, str, _ = parseMilage(block[i : i+lenIvent]) //+

		case 0xC9, 0x3C: //до 3B выше
			str, _ = parseTime(block[i : i+lenIvent]) //+

		case 0x70:
			_, str, _ = parseAcceleration(block[i : i+lenIvent]) //+

		case 0x90:
			str = fmt.Sprintf("\tДополнительная регистрация")

		case 0xF8:
			// 0xF8 (доп. параметры, данные зависят от модификатора (3-й байт после ИД))
			mod := block[i+2]
			switch mod {
			case 0: // дата
				str, _ = parseDate(block[i : i+lenIvent])
			case 1: // таб. номер
			case 3: // номер поезда
			case 12: // число оборотов ДГУ
				_, str, _ = parseFreqDGU(block[i : i+lenIvent]) //+
			case 13: // состояние доп. входа
				_, str, _ = parseAddSignal(block[i : i+lenIvent]) //+
			}

		case 0xFF:
			// ничего не делать

		default:
			str = fmt.Sprintf("\t%s (ИД 0x%X)", mapIvent[id].description, id)
		}

		if str != "" {
			strDebug = append(strDebug, str)
		}
	}

	return
}

// Изменяет глобальную переменную времени
// gTime = time.Date(gTime.Year(), gTime.Month(), gTime.Day(), gTime.Hour(), gTime.Minute(), gTime.Second(), 0, time.UTC)
func parseTime(data []byte) (str string, err error) {
	if data == nil {
		err = fmt.Errorf("Ошибка parseTime(): данные = nil")
		str = err.Error()
		return
	}

	// 0 -- 3B относительная регистрация времени 1-60 сек (те 0 это прибавить 1 сек)
	if data[0] <= 0x3B {
		if len(data) != 1 {
			err = fmt.Errorf("Ошибка parseTime(): неверная длинна (ID: 0x%X, len: %d)", data[0], len(data))
			str = err.Error()
			return
		}
		gTime = gTime.Add(time.Second * (time.Duration(data[0]) + 1))
	}

	// 3C относительная регистрация времени 60-255 сек
	if data[0] == 0x3C {
		if len(data) != 2 {
			err = fmt.Errorf("Ошибка parseTime(): неверная длинна (ID: 0x%X, len: %d)", data[0], len(data))
			str = err.Error()
			return
		}
		gTime = gTime.Add(time.Second * (time.Duration(data[1]))) // ??? Нулевому значению операнда соответствует 256 c.
	}

	// С9 нормальное время
	if data[0] == 0xC9 {
		if len(data) != 4 {
			err = fmt.Errorf("Ошибка parseTime(): неверная длинна (ID: 0x%X, len: %d)", data[0], len(data))
			str = err.Error()
			return
		}
		gTime = time.Date(gTime.Year(), gTime.Month(), gTime.Day(), int(data[1]), int(data[2]), int(data[3]), 0, time.Local)
	}

	str = fmt.Sprintf("ВРЕМЯ = %s (data: %X)", gTime.Format("15:04:05"), data)

	return
}

// - 0 – дата (1 байт – день месяца, 2 – байт номер месяца, 3 байт – номер года от 2000 года); - 6 байт
func parseDate(data []byte) (str string, err error) {

	if data[0] == 0xF8 || // доп парам
		len(data) == (int(data[1])+1) || // длина указывается без ИД, поэтому приб. 1 (6 = ИД, мод, дл, 3 байта данных)
		data[2] == 0x00 { // модификатор даты

		d := int(data[3])
		m := time.Month(data[4])
		y := 2000 + int(data[5])
		gTime = time.Date(y, m, d, gTime.Hour(), gTime.Minute(), gTime.Second(), gTime.Nanosecond(), time.Local)

		str = fmt.Sprintf("ДАТА = %s (date: %X)", gTime.Format("02.01.2006 15:04:05"), data)

	} else {
		err = fmt.Errorf("Ошибка parseData()")
		str = err.Error()
	}

	return
}

/*
Двухбайтовой сообщение 0х67
Разряд	Огонь
0x01	Зеленый 61
0x02	Желтый 62
0x04	Желтый с красным 63
0x08	Красный 64
0x10	Белый 65
*/

func parseALS(data []byte) (str string, err error) {

	if data == nil {
		err = errors.New("Ошибка parseALS(): данные == nil")
		str = err.Error()
		return
	}

	id := uint(data[0])
	if id == 0x67 && len(data) != 2 {
		err = errors.New("Ошибка parseALS(): ИД 0х67, длинна события не равна 2 байта")
		str = err.Error()
		return
	}

	if id == 0x67 {
		// пришло 2х байтовое сообщение
		switch data[1] {
		case 0x01:
			id = uint(0x61) // просто заменяю код таким же ИД, чтобы легче было проверить итоговый слайс
		case 0x02:
			id = uint(0x62)
		case 0x04:
			id = uint(0x63)
		case 0x08:
			id = uint(0x64)
		case 0x10:
			id = uint(0x65)
		}
	}

	mapALS[id] = mapIvent[id]
	str = "\t" + mapIvent[id].description

	return
}

// получить значение скорости в км/ч
func parseSpeed(data []byte) (s float64, str string, err error) {
	if data == nil {
		err = errors.New("Ошибка parseSpeed(): данные == nil")
		str = err.Error()
		return
	}

	var val byte
	id := data[0]

	if id == 0x49 || id == 0x4A || id == 0xDD || id == 0xDC { // двухбайтовые ИД
		if len(data) != 2 {
			err = errors.New("Ошибка parseSpeed(): длинна события не равно 2 байта")
			str = err.Error()
			return
		}
	} else if id >= 0x41 && id <= 48 || // 41 - 48 однобайтовый ИД
		id >= 0xD8 && id <= 0xDB { // D8 - DB
		if len(data) != 1 {
			err = errors.New("Ошибка parseSpeed(): длинна события не равно 1 байт")
			str = err.Error()
		}
	}

	if len(data) == 2 {
		val = data[1]
	}

	switch id {
	case 0x41:
		gSpeed += 1 // целое значение скорости + 1 км/ч
	case 0x42:
		gSpeed += 2 // целое значение скорости + 2 км/ч
	case 0x43:
		gSpeed += 3 // целое значение скорости + 3 км/ч
	case 0x44:
		gSpeed += 4 // целое значение скорости + 4 км/ч
	case 0x45:
		if gSpeed >= 1 {
			gSpeed -= 1 // целое значение скорости - 1 км/ч
		}
	case 0x46:
		if gSpeed >= 2 {
			gSpeed -= 2 // целое значение скорости - 2 км/ч
		}
	case 0x47:
		if gSpeed >= 3 {
			gSpeed -= 3 // целое значение скорости - 3 км/ч
		}
	case 0x48:
		if gSpeed >= 4 {
			gSpeed -= 4 // целое значение скорости - 4 км/ч
		}
	case 0x49:
		gSpeed = float64(val) // целое значение скорости < 255 км/ч
	case 0x4A:
		gSpeed = float64(val) + 256 // целое значение скорости > 255 км/ч
	case 0xD8:
		gSpeed += 0.5
	case 0xD9:
		gSpeed += 1.5
	case 0xDA:
		if gSpeed >= 0.5 {
			gSpeed -= 0.5
		}
	case 0xDB:
		if gSpeed >= 1.5 {
			gSpeed -= 1.5
		}
	case 0xDC:
		gSpeed = float64(val) + 0.5 //  не целое значение скорости < 255 км/ч
	case 0xDD:
		gSpeed = float64(val) + 256.5 //  не целое значение скорости < 255.5 км/ч
	default:
	}

	str = fmt.Sprintf("\tСкорость: %0.1f км/ч", gSpeed)

	if gSpeed != 0 {
		fSpeedSlice = append(fSpeedSlice, gSpeed)
	}

	return
}

// Давление в тормозной магистрали (тормозном цилиндре) в кгс/см2 (атм)
func parsePress(data []byte) (str string, err error) {

	if data == nil {
		err = errors.New("Ошибка parsePress(): данные == nil")
		str = err.Error()
		return
	}

	if data[0] == 0x57 && len(data) != 2 {
		err = errors.New("Ошибка parsePress(): ИД 0x57, длинна не равно 2 байта")
		str = err.Error()
		return
	}

	id := data[0]

	switch id {

	case 0x51:
		gPress += 0.1 // относительная регистрация, меняет основное значение
		str = fmt.Sprintf("\tДавление: %0.1f атм (0x%X +0.1)", gPress, id)
	case 0x52:
		gPress += 0.2
		str = fmt.Sprintf("\tДавление: %0.1f атм (0x%X +0.2)", gPress, id)
	case 0x53:
		gPress += 0.3
		str = fmt.Sprintf("\tДавление: %0.1f атм (0x%X +0.3)", gPress, id)
	case 0x54:
		gPress -= 0.1
		str = fmt.Sprintf("\tДавление: %0.1f атм (0x%X -0.1)", gPress, id)
	case 0x55:
		gPress -= 0.2
		str = fmt.Sprintf("\tДавление: %0.1f атм (0x%X -0.2)", gPress, id)
	case 0x56:
		gPress -= 0.3
		str = fmt.Sprintf("\tДавление: %0.1f атм (0x%X -0.3)", gPress, id)
	case 0x57:
		p := float64(data[1]) / 10 // рег. давления в (0.1 кгс/см2) > 0.3 ТЦ/ТМ (2 байта с ИД)
		gPress = p                 // явная регистрация
		str = fmt.Sprintf("\tДавление: %0.1f атм (0x%X - явная регистрация)", gPress, id)
	}

	if gPress != 0 {
		fPressSlice1 = append(fPressSlice1, gPress)
	}
	return
}

// Дополнительные давления (аналоговые сигналы)
func parseAnalogSignal(data []byte) ( /*number int, fval float64,*/ str string, err error) {

	if (data[0] == 0xD7 && len(data) != 3) || (data[0] != 0xD7 && len(data) != 2) {
		err = errors.New("Ошибка parseAnalogSignal()")
		str = err.Error()
		return
	}

	id := data[0]
	number := int(data[1]) // номер канала 1 - 16

	switch id {
	case 0xD1:
		if 1 == number {
			gAnalogSignal1 += 0.1
			str = fmt.Sprintf("\tAналоговый сигнал 1: %0.1f атм (0x%X +0.1)", gAnalogSignal1, id)
		} else if 2 == number {
			gAnalogSignal2 += 0.1
			str = fmt.Sprintf("\tAналоговый сигнал 2: %0.1f атм (0x%X +0.1)", gAnalogSignal2, id)
		} else if 3 == number {
			gAnalogSignal3 += 0.1
			str = fmt.Sprintf("\tAналоговый сигнал 3: %0.1f атм (0x%X +0.1)", gAnalogSignal3, id)
		} else {
			str = fmt.Sprintf("\tAналоговый сигнал №%d: ? атм (0x%X +0.1)", number, id)
		}
	case 0xD2:
		if 1 == number {
			gAnalogSignal1 += 0.2
			str = fmt.Sprintf("\tAналоговый сигнал 1: %0.1f атм (0x%X +0.2)", gAnalogSignal1, id)
		} else if 2 == number {
			gAnalogSignal2 += 0.2
			str = fmt.Sprintf("\tAналоговый сигнал 2: %0.1f атм (0x%X +0.2)", gAnalogSignal2, id)
		} else if 3 == number {
			gAnalogSignal3 += 0.2
			str = fmt.Sprintf("\tAналоговый сигнал 3: %0.1f атм (0x%X +0.2)", gAnalogSignal3, id)
		} else {
			str = fmt.Sprintf("\tAналоговый сигнал №%d: ? атм (0x%X +0.2)", number, id)
		}
	case 0xD3:
		if 1 == number {
			gAnalogSignal1 += 0.3
			str = fmt.Sprintf("\tAналоговый сигнал 1: %0.1f атм (0x%X +0.3)", gAnalogSignal1, id)
		} else if 2 == number {
			gAnalogSignal2 += 0.3
			str = fmt.Sprintf("\tAналоговый сигнал 2: %0.1f атм (0x%X +0.3)", gAnalogSignal2, id)
		} else if 3 == number {
			gAnalogSignal3 += 0.3
			str = fmt.Sprintf("\tAналоговый сигнал 3: %0.1f атм (0x%X +0.3)", gAnalogSignal3, id)
		} else {
			str = fmt.Sprintf("\tAналоговый сигнал №%d: ? атм (0x%X +0.3)", number, id)
		}
	case 0xD4:
		if 1 == number {
			gAnalogSignal1 -= 0.1
			str = fmt.Sprintf("\tAналоговый сигнал 1: %0.1f атм (0x%X -0.1)", gAnalogSignal1, id)
		} else if 2 == number {
			gAnalogSignal2 -= 0.1
			str = fmt.Sprintf("\tAналоговый сигнал 2: %0.1f атм (0x%X -0.1)", gAnalogSignal2, id)
		} else if 3 == number {
			gAnalogSignal3 -= 0.1
			str = fmt.Sprintf("\tAналоговый сигнал 3: %0.1f атм (0x%X -0.1)", gAnalogSignal3, id)
		} else {
			str = fmt.Sprintf("\tAналоговый сигнал №%d: ? атм (0x%X -0.1)", number, id)
		}
	case 0xD5:
		if 1 == number {
			gAnalogSignal1 -= 0.2
			str = fmt.Sprintf("\tAналоговый сигнал 1: %0.1f атм (0x%X -0.2)", gAnalogSignal1, id)
		} else if 2 == number {
			gAnalogSignal2 -= 0.2
			str = fmt.Sprintf("\tAналоговый сигнал 2: %0.1f атм (0x%X -0.2)", gAnalogSignal2, id)
		} else if 3 == number {
			gAnalogSignal3 -= 0.2
			str = fmt.Sprintf("\tAналоговый сигнал 3: %0.1f атм (0x%X -0.2)", gAnalogSignal3, id)
		} else {
			str = fmt.Sprintf("\tAналоговый сигнал №%d: ? атм (0x%X -0.2)", number, id)
		}
	case 0xD6:
		if 1 == number {
			gAnalogSignal1 -= 0.3
			str = fmt.Sprintf("\tAналоговый сигнал 1: %0.1f атм (0x%X -0.3)", gAnalogSignal1, id)
		} else if 2 == number {
			gAnalogSignal2 -= 0.3
			str = fmt.Sprintf("\tAналоговый сигнал 2: %0.1f атм (0x%X -0.3)", gAnalogSignal2, id)
		} else if 3 == number {
			gAnalogSignal3 -= 0.3
			str = fmt.Sprintf("\tAналоговый сигнал 3: %0.1f атм (0x%X -0.3)", gAnalogSignal3, id)
		} else {
			str = fmt.Sprintf("\tAналоговый сигнал №%d: ? атм (0x%X -0.3)", number, id)
		}
	case 0xD7:
		fval := float64(data[2]) / 10 // давлениe в (0.1 кгс/см2) > 0.3 (3 байта с ИД)
		if 1 == number {
			gAnalogSignal1 = fval
			str = fmt.Sprintf("\tAналоговый сигнал 1: %0.1f атм (0x%X явная регистрация)", gAnalogSignal1, id)
		} else if 2 == number {
			gAnalogSignal2 = fval
			str = fmt.Sprintf("\tAналоговый сигнал 2: %0.1f атм (0x%X явная регистрация)", gAnalogSignal2, id)
		} else if 3 == number {
			gAnalogSignal3 = fval
			str = fmt.Sprintf("\tAналоговый сигнал 3: %0.1f атм (0x%X явная регистрация)", gAnalogSignal3, id)
		} else {
			str = fmt.Sprintf("\tAналоговый сигнал №%d: %0.1f атм (0x%X явная регистрация)", number, fval, id)
		}
	}

	if (1 == number) && (gAnalogSignal1 != 0) {
		fPressSlice2 = append(fPressSlice2, gAnalogSignal1)
	} else if (2 == number) && (gAnalogSignal2 != 0) {
		fPressSlice3 = append(fPressSlice3, gAnalogSignal2)
	}

	return
}

func parseMilage(data []byte) (m uint64, str string, err error) {

	if len(data) != 4 {
		err = errors.New("Ошибка parseMilage()")
		str = err.Error()
		return
	}

	m = (uint64(data[3]) << 16) | (uint64(data[2]) << 8) | (uint64(data[1]))
	str = fmt.Sprintf("\tПробег: %d м", m)

	uMilageSlice = append(uMilageSlice, m)
	return
}

func parseAcceleration(data []byte) (a float64, str string, err error) { //+
	if len(data) != 2 {
		err = errors.New("Ошибка parseAcceleration()") //+
		str = err.Error()
		return
	}

	a = float64(data[1]) / 100 // модуль ускорения в 0,01 м/с2, ускорение только отрицательное
	str = fmt.Sprintf("\tУскорение %0.3f м/с2", a)

	fAccelSlice = append(fAccelSlice, a)

	return
}

// число оборотов ДГУ (об/мин)
// 0xF8 | len | Mod | DATA
// f8 04 0c 4a 01 -> ЧАСТОТА ВРАЩЕНИЯ ВД = 330 об/мин
func parseFreqDGU(data []byte) (f uint64, str string, err error) {

	if data[0] == 0xF8 || // доп парам
		len(data) == (int(data[1])+1) || // длина без ID
		data[2] == 0x0C { // модификатор частоты ДГУ

		f = uint64(data[4])*256 + uint64(data[3])
		str = fmt.Sprintf("\tЧастота вращения ВД = %d об/мин", f)

		if f != 0 {
			uFreqDGUSlice = append(uFreqDGUSlice, f)
		}
	} else {
		err = errors.New("Ошибка parseFreqDGU()")
		str = err.Error()
	}

	return
}

// дополнительный сигнал
// 0xF8 | len | Mod | DATA
// f8 04 0D 01 01 -> f8 04 0D 01 00 судя по всему под данные отведено 2 байта и нужный бит в последнем байте
func parseAddSignal(data []byte) (f uint64, str string, err error) {

	if data[0] == 0xF8 || // доп парам
		len(data) == (int(data[1])+1) || // длина без ID
		data[2] == 0x0D { // модификатор

		l := len(data)
		str = fmt.Sprintf("\tДополнительный сигнал: %X", data[l-1])
		// fmt.Println(str)
		uAddSignal = append(uAddSignal, data[l-1])
	} else {
		err = errors.New("Ошибка parseFreqDGU()")
		str = err.Error()
	}

	return
}

// Сигналы пожарной безопасности
func parseFireAlarmSignals(data []byte) (str string, err error) {

	if data[0] != 0xAD || len(data) != 3 {
		err = errors.New("Ошибка parseFireAlarmSignals()")
		str = err.Error()
		return
	}
	uFireAlarmSlice1 = append(uFireAlarmSlice1, data[1])
	uFireAlarmSlice2 = append(uFireAlarmSlice2, data[2])

	str = fmt.Sprintf("\tСПБ1К = 0x%X, СПБ2К = 0x%X", data[1], data[2])

	return
}

// Сбои БУ
func parseFaults(data []byte) (str string, err error) {
	var code uint16

	if data[0] == 0xB0 || // ИД
		len(data) == (int(data[1])*2+1) { // длина всего сообщение

		n := data[1]      // число сбоев в сообщении (каждый сбой 2 байта)
		l := int(n)*2 + 2 // длинна

		for i := 2; i < l; i += 2 {
			code = uint16(data[i]) + uint16(data[i+1])*256 // код сбоя младшим байтом вперед
			if code != 0 {
				uFaults = append(uFaults, code)
			}
		}

		str = fmt.Sprintf("\tСБОЙ = %d", code)

	} else {
		err = errors.New("Ошибка parseFaults()")
		str = err.Error()
	}

	return
}
