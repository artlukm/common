package parsebin

import (
	"common"
	"errors"
	"fmt"
	"io"
	"ipkwiz"
	"math"
	"os"
	"time"
)

var wiz *ipkwiz.Wizard

// InitCheckBIN init
func InitCheckBIN(w *ipkwiz.Wizard) (err error) {
	initAllMaps()

	if w == nil {
		err = errors.New("initCheckBin(): wiz == nil")
		return
	}
	wiz = w

	return
}

// CheckBIN основная
func CheckBIN(d DataForCheck) (ok bool) {
	ok = true

	// Проверка введенных данных
	if d.NameBIN == "" {
		wiz.Error("Не указано имя и путь файла для анализ")
	}
	if !d.It.StageCheckMileage && !d.It.StageCheckSpeed && !d.It.StageCheckFrequencyDGU &&
		!d.It.StageCheckAcceleration && !d.It.StageCheckPress && !d.It.StageCheckALS &&
		!d.It.StageCheckCode && !d.It.StageBUSM {
		wiz.Error("Не выбран ни один этап проверки")
	}
	if d.ScaleLimit == 0 {
		d.ScaleLimit = 150
	}

	createMapIvent()

	rd, err := os.Open(d.NameBIN)
	defer rd.Close()
	if err != nil {
		fmt.Printf("Ошибка открытия файла: %v", err)
		return
	}

	var wr *os.File
	if d.FileReport {
		nameReport := common.GetOwnNameFolder(1) + "\\compare\\report.txt"
		wr, err = os.Create(nameReport) //файл отладки переместить todo
		defer wr.Close()
		if err != nil {
			fmt.Printf("Ошибка открытия файла для записи: %v", err)
			return
		}
		wr.WriteString(fmt.Sprintf("Данные полученные из файла: %s (%v)\r\n", d.NameBIN, time.Now().Format("02.01.2006 15:04:05")))
		wr.WriteString("\r\n")
	}

	//---------------------------------------- анализ bin-файла ----------------------------------------//

	var tail []byte // если данные не умещаются в одном блоке часть их записывается в следующий (например id может быть в одном блоке, а данные в другом) сюда запишем то что нужно склеить с данными из следующего блока

	_, offset := findHeader(d.NumberTrips, rd)
	rd.Seek(offset, 0)

	for {
		block, err := readNextBytes(rd, sizeBlock) // начинаем читать с шапки headerNumber по смещению offset
		if err == io.EOF {
			fmt.Println("Конец файла")
			break
		}

		if ifEmptyBlock(block) {
			if d.FileReport {
				wr.WriteString(fmt.Sprintf("Пустой блок разделитель \r\n"))
			}
			tail = nil // todo у текущей версии БУ-4 (12.2020) есть такой косяк: неполные сообщения перед пустым блоком/шапкой (а сама шапка вместо доп рег)
		} else {

			if len(tail) > 0 {
				block = append(tail, block...) // если есть хвост от предыдущего блока, добавить вперед
				tail = nil                     // очистить
			}

			// вот тут наконец разбор считанного куска:
			strDebug, iTail, _ := parseBlock(block)
			if iTail > 0 {
				l := len(block) - 4 // последние 4 байта КС контрольная сумма
				tail = block[iTail:l]
			}
			if d.FileReport {
				writeSliceToFile(wr, strDebug)
			}
		}
	}

	// вывести все значения перед выходом
	if d.FileReport {
		writeAllDataToFile(wr)
	}

	//---------------------------------------- проверка полученных данных ----------------------------------------//

	if d.It.StageCheckSpeed {
		if !checkSpeed(d.ScaleLimit, d.Block) {
			ok = false
		}
	}
	if d.It.StageCheckFrequencyDGU {
		if !checkFrequencyDGU(d.ScaleLimit, d.BandageDiameter, d.NumberTeeth, d.NumberOfPulses) {
			ok = false
		}
	}
	if d.It.StageCheckPress {
		if !checkPress(d.Block, d.PressureLimit) {
			ok = false
		}
	}
	if d.It.StageCheckMileage {
		if !checkMilage() {
			ok = false
		}
	}
	if d.It.StageCheckAcceleration {
		if !checkAccel() {
			ok = false
		}
	}
	if d.It.StageCheckALS {
		if !checkALS(d.Block) {
			ok = false
		}
	}
	if d.It.StageCheckCode {
		if !checkPeriodAndCode() {
			ok = false
		}
	}
	if d.It.StageBUSM {
		if !checkBUSM() {
			ok = false
		}
		if !checkCrane395() {
			ok = false
		}
		if !checkFireAlarm() {
			ok = false
		}
	}

	fmt.Println("DONE")

	return
}

//---------------------------------------- ------------------ ----------------------------------------//

func checkSpeed(scaleLimit int, block BlockType) bool {
	controlSpeed := []int{5, 10, 20, 30, 50, 75, 100, 150, 300} // нули не записываю в fSpeedSlice для экономии todo 300 есть в
	registerSpeed := make(map[int]bool)
	const MAXERROR1 = 0.1
	const MAXERROR2 = 10.
	okSpeed := true

	wiz.Msg("Проверка скорости")
	time.Sleep(500 * time.Millisecond)

	maxError := MAXERROR1
	for _, controlVal := range controlSpeed {

		if controlVal >= 10 {
			maxError = MAXERROR2
		}
		for _, val := range fSpeedSlice {
			if (val < (float64(controlVal) + maxError)) && (val > (float64(controlVal) - maxError)) {
				registerSpeed[controlVal] = true
				break
			}
		}
	}
	for _, controlVal := range controlSpeed {

		if controlVal > scaleLimit {
			break // на блоках БУ не проверяется скорость 300
		}

		if registerSpeed[controlVal] {
			wiz.ButtonOk(fmt.Sprintf("Скорость: %d км/ч — зарегистрирована", controlVal))
		} else {
			wiz.Error(fmt.Sprintf("Скорость: %d км/ч — не зарегистрирована", controlVal))
			wiz.Msg(" ")
			okSpeed = false
		}
	}

	if okSpeed {
		wiz.ButtonOk("Скорость — зарегистрирована")
	}

	// Дополнительные проверки на этапе скорость
	okAddCheck := true
	if block == BlockBU3PS {
		// проверяем коды ошибок 114 124
		h114, h124 := false, false
		for v := range uFaults {
			if v == 114 {
				h114 = true
			}
			if v == 124 {
				h124 = true
			}
		}
		if !h114 {
			wiz.Error("Не зарегистрирован код ошибки H114")
			okAddCheck = false
		}
		if !h124 {
			wiz.Error("Не зарегистрирован код ошибки H124")
			okAddCheck = false
		}
	}

	return okSpeed && okAddCheck
}

func checkFrequencyDGU(scale, diameter, nteeth, npulses int) (ok bool) {
	controlSpeed := []int{5, 10, 20, 30, 50, 75, 100, 150, 300}
	// controlFreq := []uint64{17, 33, 66, 99, 165, 248, 330, 495, 990}
	registerFreq := make(map[uint32]bool)
	ok = true
	const MAXERROR = 10 // +- 10 об/мин

	if diameter == 0 || nteeth == 0 || npulses == 0 {
		return true // если нет данных просто не проверяем (в отчете не будет строчки, но ошибки не будет)
	}

	wiz.Msg("Проверка частоты вращения вала ДГУ (об/мин)")

	for _, s := range controlSpeed {
		controlVal := common.RpmCalculation(s, diameter, nteeth, npulses)

		for _, val := range uFreqDGUSlice {
			if math.Abs(float64(val)-float64(controlVal)) <= MAXERROR {
				registerFreq[controlVal] = true
				break
			}
		}
	}
	for _, s := range controlSpeed {
		if (s <= scale) || (s == 300) { // скорость 300 (частота 990) устанавливается для любой шкалы
			controlVal := common.RpmCalculation(s, diameter, nteeth, npulses)

			if registerFreq[controlVal] {
				wiz.ButtonOk(fmt.Sprintf("Частота вращения ВД: %d — зарегистрирована", controlVal))
			} else {
				wiz.Error(fmt.Sprintf("Частота вращения ВД: %d — не зарегистрирована", controlVal))
				wiz.Msg(" ")
				ok = false
			}
		}
	}

	if ok {
		wiz.ButtonOk("Частота вращения ВД — зарегистрирована")
	}

	return
}

func checkPress(block BlockType, PressureLimit float64) bool {
	// на разных блоках задаются разные значения давлений
	pressureBU4 := []float64{1, 2, 4, 5, 6, 8, 10} // нули не записываю в fSpeedPress для экономии //Значения для БУ4!!! todo у него 2 канал, у 3пс 3!
	// pressureBU3 := [...]float64{0.3, 0.6, 3.0, 3.5, 4.0, 4.5, 5.0, 5.5, 6.0, 6.5}
	pressureBU3channel1 := [...]float64{0.3, 0.6, 3.0, 3.5, 4.0, 4.5, 5.0, 5.5, 6.0, 6.5, 8.0, 10.0}
	pressureBU3channel2 := [...]float64{0.3, 1.0, 2.0, 4.0, 5.0, 8.0, 10.0}
	pressureBU3PS := [...]float64{1.0, 2.0, 4.0, 5.0, 6.0, 8.0, 10.0}
	var controlPress []float64

	registerPress1 := make(map[float64]bool)
	registerPress2 := make(map[float64]bool)
	registerPress3 := make(map[float64]bool)

	const MAXERROR = 0.15
	ok1, ok2, ok3 := true, true, true

	// Канал 1
	controlPress = pressureBU4[:]
	if block == BlockBU3 {
		controlPress = pressureBU3channel1[:]
	} else if block == BlockBU3PS {
		controlPress = pressureBU3PS[:]
	}

	wiz.Msg("Проверка давления (канал 1)")
	time.Sleep(500 * time.Millisecond)

	for _, controlVal := range controlPress {

		for _, val := range fPressSlice1 {
			if (val < (controlVal + MAXERROR)) && (val > (controlVal - MAXERROR)) {
				registerPress1[controlVal] = true
				break
			}
		}
	}
	for _, controlVal := range controlPress {
		if registerPress1[controlVal] {
			wiz.ButtonOk(fmt.Sprintf("Давление (канал 1): %0.2f атм — зарегистрировано", controlVal))
		} else {
			wiz.Error(fmt.Sprintf("Давление (канал 1): %0.2f атм — не зарегистрировано", controlVal))
			wiz.Msg(" ")
			ok1 = false
		}
	}
	if ok1 {
		wiz.ButtonOk("Давление (канал 1) — зарегистрировано")
	}

	// Канал 2
	MAXERROR2 := 0.15

	if block == BlockBU3 && PressureLimit == 0 {
		return ok1 // если не указан для 3П предел относительно которого нужно делать перещет, то не проверяем 2 канал
	}

	controlPress = pressureBU4[:]
	if block == BlockBU3 {
		MAXERROR2 = PressureLimit * 1.5 / 100
		controlPress = pressureBU3channel2[:]
	} else if block == BlockBU3PS {
		controlPress = pressureBU3PS[:]
	}

	if len(fPressSlice2) > 0 { // у БУ4 может быть только один канал давления если нет данных о втором канале -- не выводить ошибку
		wiz.Msg("Проверка давления (канал 2)")
		time.Sleep(500 * time.Millisecond)

		for _, controlVal := range controlPress {
			if block == BlockBU3 {
				controlVal *= PressureLimit / 10
			}

			for _, val := range fPressSlice2 {
				if (val < (controlVal + MAXERROR2)) && (val > (controlVal - MAXERROR2)) {
					registerPress2[controlVal] = true
					break
				}
			}
		}
		for _, controlVal := range controlPress {
			if block == BlockBU3 {
				controlVal *= PressureLimit / 10
			}
			if registerPress2[controlVal] {
				wiz.ButtonOk(fmt.Sprintf("Давление (канал 2): %0.2f атм — зарегистрировано", controlVal))
			} else {
				wiz.Error(fmt.Sprintf("Давление (канал 2): %0.2f атм — не зарегистрировано", controlVal))
				wiz.Msg(" ")
				ok2 = false
			}
		}
		if ok2 {
			wiz.ButtonOk("Давление (канал 2) — зарегистрировано")
		}
	}

	// Канал 3
	if block != BlockBU3PS {
		return ok1 && ok2
	}

	if len(fPressSlice3) > 0 {
		wiz.Msg("Проверка давления (канал 3)")
		time.Sleep(500 * time.Millisecond)

		for _, controlVal := range controlPress {

			for _, val := range fPressSlice3 {
				if (val < (controlVal + MAXERROR)) && (val > (controlVal - MAXERROR)) {
					registerPress3[controlVal] = true
					break
				}
			}
		}
		for _, controlVal := range controlPress {
			if registerPress3[controlVal] {
				wiz.ButtonOk(fmt.Sprintf("Давление (канал 3): %0.2f атм — зарегистрировано", controlVal))
			} else {
				wiz.Error(fmt.Sprintf("Давление (канал 3): %0.2f атм — не зарегистрировано", controlVal))
				wiz.Msg(" ")
				ok2 = false
			}
		}
		if ok3 {
			wiz.ButtonOk("Давление (канал 3) — зарегистрировано")
		}
	}

	return ok1 && ok2 && ok3
}

func checkMilage() (ok bool) {
	checkValue := uint64(20000)

	for _, val := range uMilageSlice {
		if val >= checkValue {
			ok = true
		}
	}

	if ok {
		wiz.Success(fmt.Sprintf("Пройденный путь — зарегистрирован"))
	} else {
		wiz.Error(fmt.Sprintf("Пройденный путь: (%d м) — не зарегистрирован", checkValue))
	}
	return
}

func checkAccel() (ok bool) {
	controlAccel := []float64{0.08, 0.40, 0.52, 0.99} // в расшифровке только отрицательные ускореия и по модулю
	registerAccel := make(map[float64]bool)
	const MAXERROR = 0.02
	ok = true

	wiz.Msg("Проверка ускорения")
	time.Sleep(500 * time.Millisecond)

	for _, controlVal := range controlAccel {

		for _, val := range fAccelSlice {
			if (val <= (float64(controlVal) + MAXERROR)) && (val >= (float64(controlVal) - MAXERROR)) {
				registerAccel[controlVal] = true
				break
			}
		}
	}
	for _, controlVal := range controlAccel {
		if registerAccel[controlVal] {
			wiz.ButtonOk(fmt.Sprintf("Ускорение: %0.2f м/с2 — зарегистрировано", controlVal))
		} else {
			wiz.Error(fmt.Sprintf("Ускорение: %0.2f м/с2 — не зарегистрировано", controlVal))
			wiz.Msg(" ")
			ok = false
		}
	}

	if ok {
		wiz.ButtonOk("Ускорение — зарегистрировано")
	}

	return
}

func checkALS(block BlockType) bool {
	checkID := []uint{0x61, 0x62, 0x63, 0x64, 0x65}
	alsOK := true

	wiz.Msg("Проверка АЛС")
	time.Sleep(500 * time.Millisecond)

	for _, id := range checkID {
		if _, present := mapALS[id]; !present {
			wiz.Error(fmt.Sprintf("%s — не зарегистрирован", mapIvent[id].description))
			wiz.Msg(" ")
			alsOK = false
		} else {
			wiz.ButtonOk(fmt.Sprintf("%s — зарегистрирован", mapIvent[id].description))
		}
	}
	if alsOK {
		wiz.ButtonOk("АЛС — зарегистрированы")
	}

	//------------------------------------------------
	// проверка дополнительного сигнала (3ПС) должна быть регистрация 0 и 1

	signOk := false
	if block == BlockBU3PS {
		setSign, resetSign := false, false
		for u := range uAddSignal {
			if u == 0 {
				resetSign = true
			}
			if u == 1 {
				setSign = true
			}
		}
		signOk = resetSign && setSign
		if signOk {
			wiz.Success("Дополнительный сигнал — зарегистрирован")
		} else {
			wiz.Error("Дополнительный сигнал — не зарегистрирован")
		}
	} else {
		signOk = true
	}

	return (alsOK && signOk)
}

// коды рельсовой цепи и период кодирования
func checkPeriodAndCode() bool {
	periodOk, codesOk := false, false

	// период кодирования
	var p16, p19 int
	for _, id := range uPeriodSlice {
		switch id {
		case 0x80, 0x81, 0x82, 0x83, 0x84:
			p16++
		case 0x85, 0x86, 0x87, 0x88, 0x89:
			p19++
		} // 0x8A ошибка, отсутствие кода, не проверяется
	}

	count := 1 // просто проверяем, что период кодирования 1.6 и 1.9 встречается несколько раз
	if p16 < count {
		wiz.Error("Период кодирования 1,6 сек — не зарегистрирован")
	}
	if p19 < count {
		wiz.Error("Период кодирования 1,9 сек  — не зарегистрирован")
	}
	if (p16 >= count) && (p19 >= count) {
		wiz.Success("Периоды кодирования 1,6 и 1,9 сек — зарегистрированы")
		periodOk = true
	}

	// коды рельсовой цепи
	var noneCode, yellowAndRedCode, yellowCode, greenCode int
	for _, id := range uCodesRCSlice {
		switch id {
		case 0xA0:
			noneCode++
		case 0xA1:
			yellowAndRedCode++
		case 0xA2:
			yellowCode++
		case 0xA3:
			greenCode++
		}
	}
	if yellowAndRedCode < 1 {
		wiz.Error("Код рельсовой цепи «Желтый с красным» — не зарегистрирован")
	}
	if yellowCode < 1 {
		wiz.Error("Код рельсовой цепи «Желтый»  —  не зарегистрирован")
	}
	if greenCode < 1 {
		wiz.Error("Код рельсовой цепи «Зеленый»  — не зарегистрирован")
	}
	if yellowAndRedCode >= 1 && yellowCode >= 1 && greenCode >= 1 {
		wiz.Success("Коды рельсовой цепи — зарегистрированы")
		codesOk = true
	}

	return periodOk && codesOk
}

func checkCrane395() (ok bool) {
	count := 1 // все значения встречаются несколько раз

	var pos12, pos34, pos55a, pos6 int // отпуск, перекрыша, тормож., экст.тормож.
	var saut34, saut5 int              // перекрыша и тормож.  САУТ
	for _, id := range uCrane395Slice {
		switch id {
		case 0x58:
			pos12++
		case 0x59:
			pos34++
		case 0x5A:
			pos55a++
		case 0x5B:
			pos6++
		case 0x5C:
			saut34++
		case 0x5D:
			saut5++
		}
		// 0x5E 2 байта -- код ошибки принятый с крана 395, не проверяется
	}
	if pos12 < count {
		wiz.Error("Позиция 395 (отпуск) — не зарегистрирована")
	}
	if pos34 < count {
		wiz.Error("Позиция 395 (перекрыша) — не зарегистрирована")
	}
	if pos55a < count {
		wiz.Error("Позиция 395 (торможениe) — не зарегистрирована")
	}
	if pos6 < count {
		wiz.Error("Позиция 395 (экстр. торможение) — не зарегистрирована")
	}
	if saut34 < count {
		wiz.Error("Позиция 395 (перекрыша  САУТ) — не зарегистрирована")
	}
	if saut5 < count {
		wiz.Error("Позиция 395 (торможение САУТ) — не зарегистрирована")
	}
	if pos12 >= count && pos34 >= count && pos55a >= count && pos6 >= count &&
		saut34 >= count && saut5 >= count {
		wiz.Success("Позиции крана 395 — зарегистрированы")
		ok = true
	}

	return
}

func checkBUSM() (ok bool) {
	checkID := []uint{0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7A, 0x7B, 0X7C, 0x7D,
		0x7E, 0x91, 0x92, 0xA5, 0xA6 /*0xAB, 0xAC,*/, 0xA9, 0xAA} // САУТ не зарегистрировался
	ok = true

	wiz.Msg("Проверка сигналов БУС-М")
	time.Sleep(500 * time.Millisecond)

	for _, id := range checkID {
		if _, present := mapBUSM[id]; !present {
			wiz.Error(fmt.Sprintf("%s — не зарегистрирован", mapIvent[id].description))
			wiz.Msg(" ")
			ok = false
		} else {
			wiz.ButtonOk(fmt.Sprintf("%s — зарегистрирован", mapIvent[id].description))
		}
	}
	if ok {
		wiz.ButtonOk("Сигналы БУС-М — зарегистрированы")
	} else {
		wiz.Error("Сигналы БУС-М не зарегистрированы")
	}

	return
}

func checkFireAlarm() (ok bool) {
	ok = true
	// 1 кабина(1 байт: 0 бит -> 3 бит): ПС ВКЛ=1,  ПС АВТ=1, 	 ПС не АКТ=0, СПТ АКТ=1 (=0B)
	// 1 кабина(1 байт: 0 бит -> 3 бит): ПС ВЫКЛ=0, ПС РУЧНОЙ=0, ПС АКТ=1,    СПТ не АКТ=0 (=04)

	state1, state2 := byte(0x0B), byte(0x04)
	ok1, ok2 := false, false
	for _, val := range uFireAlarmSlice1 {
		if state1 == val {
			ok1 = true
		}
		if state2 == val {
			ok2 = true
		}
		if ok1 && ok2 {
			break
		}
	}
	if !ok1 {
		wiz.Error("Сигнал: ПОЖАРНАЯ СИГНАЛИЗАЦИЯ = (КАБИНА1: ВКЛ., АВТО, СРАБАТЫВАНИЕ ПОЖАРОТУШЕНИЯ) — не зарегистрирован")
		ok = false
	}
	if !ok2 {
		wiz.Error("Сигнал: ПОЖАРНАЯ СИГНАЛИЗАЦИЯ = (КАБИНА1: ВЫКЛ., РУЧНОЙ, СРАБАТЫВАНИЕ СИГНАЛИЗАЦИИ) — не зарегистрирован")
		ok = false
	}

	// 2 кабина(2 байт: 0 бит -> 3 бит): ПС ВЫКЛ=0, ПС АВТ=1, 	 ПС АКТ=1,    СПТ не АКТ=0 (=06)
	// 2 кабина(2 байт: 0 бит -> 3 бит): ПС ВКЛ=1,  ПС РУЧНОЙ=0, ПС не АКТ=0, СПТ АКТ=1 (=09)

	state1, state2 = byte(0x06), byte(0x09)
	ok1, ok2 = false, false
	for _, val := range uFireAlarmSlice2 {
		if state1 == val {
			ok1 = true
		}
		if state2 == val {
			ok2 = true
		}
		if ok1 && ok2 {
			break
		}
	}
	if !ok1 {
		wiz.Error("Сигнал: ПОЖАРНАЯ СИГНАЛИЗАЦИЯ = (КАБИНА2: ВЫКЛ., АВТО, СРАБАТЫВАНИЕ СИГНАЛИЗАЦИИ) — не зарегистрирован")
		ok = false
	}
	if !ok2 {
		wiz.Error("Сигнал: ПОЖАРНАЯ СИГНАЛИЗАЦИЯ = (КАБИНА2: ВКЛ., РУЧНОЙ, СРАБАТЫВАНИЕ ПОЖАРОТУШЕНИЯ) — не зарегистрирован")
		ok = false
	}

	if ok {
		wiz.Success("Сигналы пожарной сигнализации — зарегистрированы")
	}

	return
}
