package parsebin

import (
	"fmt"
	"os"
)

/*	-------------------- чтение и запись -------------------- */

func readNextBytes(file *os.File, number int) ([]byte, error) {

	bytes := make([]byte, number)
	_, err := file.Read(bytes)

	return bytes, err
}

func writeSliceToFile(file *os.File, strSlice []string) {

	for i := 0; i < len(strSlice); i++ {
		file.WriteString(strSlice[i] + "\r\n")
	}
}

func writeAllDataToFile(file *os.File) { // отладка

	file.WriteString("\r\n")
	file.WriteString("Все найденные данные для анализа:\r\n")
	file.WriteString("\r\n")

	file.WriteString("Скорость (км/ч):\r\n")
	for _, f := range fSpeedSlice {
		if f != 0 {
			s := fmt.Sprintf("%5.1f; ", f)
			file.WriteString(s)
		}
	}
	file.WriteString("\r\n")

	file.WriteString("\r\nЧастота вращения об/мин):\r\n")
	for _, u := range uFreqDGUSlice {
		if u != 0 {
			s := fmt.Sprintf("%4d; ", u)
			file.WriteString(s)
		}
	}
	file.WriteString("\r\n")

	file.WriteString("\r\nДавление 1 (атм):\r\n")
	for _, f := range fPressSlice1 {
		if f != 0 {
			s := fmt.Sprintf("%5.1f; ", f)
			file.WriteString(s)
		}
	}
	file.WriteString("\r\nДавление 2 (атм):\r\n")
	for _, f := range fPressSlice2 {
		if f != 0 {
			s := fmt.Sprintf("%5.1f; ", f)
			file.WriteString(s)
		}
	}
	file.WriteString("\r\nДавление 3 (атм):\r\n")
	for _, f := range fPressSlice3 {
		if f != 0 {
			s := fmt.Sprintf("%5.1f; ", f)
			file.WriteString(s)
		}
	}
	file.WriteString("\r\n")

	/*file.WriteString("\r\nПробег (м):\r\n")
	for _, v := range uMilageSlice {
		if v != 0 {
			s := fmt.Sprintf("%5.0d; ", v)
			file.WriteString(s)
		}
	}*/

	file.WriteString("\r\nУскорение (м/c2):\r\n")
	for _, f := range fAccelSlice {
		if f != 0 {
			s := fmt.Sprintf("%5.2f; ", f)
			file.WriteString(s)
		}
	}
	file.WriteString("\r\n")

	file.WriteString("\r\nАЛС:\r\n")
	for _, ivent := range mapALS {
		file.WriteString(ivent.description + ";\r\n")
	}

	file.WriteString("\r\nДополнительный сигнал:\r\n")
	for _, u := range uAddSignal {
		s := fmt.Sprintf("%d", u)
		file.WriteString(s + " ;")
	}
	file.WriteString("\r\n")

	file.WriteString("\r\nПериод кодирования:\r\n")
	for _, u := range uPeriodSlice {
		file.WriteString(mapIvent[u].description + ";\r\n")
	}

	file.WriteString("\r\nКоды рельсовой цепи:\r\n")
	for _, u := range uCodesRCSlice {
		file.WriteString(mapIvent[u].description + ";\r\n")
	}

	file.WriteString("\r\nПозиции крана 395:\r\n")
	for _, u := range uCrane395Slice {
		file.WriteString(mapIvent[u].description + ";\r\n")
	}

	file.WriteString("\r\nСигналы БУС-М:\r\n")
	for _, ivent := range mapBUSM {
		file.WriteString(ivent.description + ";\r\n")
	}

	file.WriteString("\r\nСигналы пожарной безопасности первой кабины локомотива:\r\n")
	for _, u := range uFireAlarmSlice1 {
		s := fmt.Sprintf("0x%X", u)
		file.WriteString(s + ";\r\n")
	}

	file.WriteString("\r\nСигналы пожарной безопасности второй кабины локомотива:\r\n")
	for _, u := range uFireAlarmSlice2 {
		s := fmt.Sprintf("0x%X", u)
		file.WriteString(s + ";\r\n")
	}

	file.WriteString("\r\nКоды сбоев БУ:\r\n")
	for _, u := range uFaults {
		s := fmt.Sprintf("%d", u)
		file.WriteString(s + "; ")
	}
	file.WriteString("\r\n")
}
