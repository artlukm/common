package parsebin

import "time"

// sizeBlock размер блока данных
const sizeBlock = 64
const sizeHeadBlock = 26

// BlockType варианты проверяемых блоков (тк разные проверяемые данные БУ-4 / БУ-3 разный набор скоростей, например)
type BlockType int

const (
	// BlockBU4 БУ-4 блок
	BlockBU4 BlockType = 1 + iota
	// BlockBU3 блоки БУ-3П, БУ-3ПА, БУ-3ПВ не отличаются по проверяемым данным
	BlockBU3
	// BlockBU3PS БУ-3ПС
	BlockBU3PS
	BlockBSSN
)

// DataForCheck входные параметры
type DataForCheck struct {
	FileReport      bool      // нужно ли записывать файл с расшифровкой
	Block           BlockType // вариант блока
	NameBIN         string    // имя файла для проверки
	It              Items     // этапы для проверки
	ScaleLimit      int       // предел шкалы нужен для проверок частоты ДГУ, скорости
	PressureLimit   float64   // верхний предел измерения давления в ТЦ, нужен для проверки давления в канале 2 у БУ-3(П/ПА/ПВ)
	BandageDiameter int       // диаметр бандажа первой колесной пары, нужен для проверки частоты ДГУ у 3ПС
	NumberTeeth     int       // число зубьев модулятора ДУП, нужен для проверки частоты ДГУ у 3ПС
	NumberOfPulses  int       // число импульсов на один оборот вала, нужен для проверки частоты ДГУ у 3ПС
	NumberTrips     int       // количество поездок, которые нужно взять с конца bin файла
}

// Items в ini файлах скриптов похожие этапы могут называться по разному (контроль скорости/скоростей), поэтому своя map
type Items struct {
	StageCheckMileage      bool
	StageCheckSpeed        bool
	StageCheckFrequencyDGU bool
	StageCheckAcceleration bool
	StageCheckPress        bool
	StageCheckALS          bool
	StageCheckCode         bool //коды рельсовой цепи и период кодирования
	StageBUSM              bool
}

// я не знаю в чем тут прикол но поля должны быть с заглавной буквы иначе не работает чтение binary.Read
type timeBU struct {
	Hour byte // Часы		(данные:) 0-23	(кол-во байт:) 1
	Min  byte // Минуты	0-59	1
	Sec  byte // Секунды	0-59	1
}

// вообще то шапка
type head struct {
	Marker [3]byte // Маркер файла поездки	(данные/формат:) DAh, DBh, DCh	(кол-во байт:) 3
	// Marker2          byte
	// Marker3          byte
	TripNumber       uint16 // Номер поездки в модуле памяти	Целое	2
	Milage           uint32 // Общий пробег локомотива, км		Целое	4
	Number100m       byte   // Стометровок последнего километра	Целое	1  -- это вообще что todo
	MetersOfLast100m uint16 // Метров последней стометровки	Один байт после точки	2 -- и это todo
	TypeLoco         uint16 // Код серии локомотива				Целое	2
	NumberLoco       uint16 // Номер локомотива					Целое	2
	Cab              byte   // Признак одно / двухкабинного локомотива или МВПС	(1, 2, 3)	1
	AlsCode          byte   // Код варианта системы АЛС			Целое	1
	PresentEPT       byte   // Признак наличия ЭПТ			00h, 0FFh	1
	SpeedY           byte   // Уставка Vж, км/ч				0 – 255		1
	SpeedRY          byte   // Уставка Vкж, км/ч			0 – 255		1
	SpeedU1          byte   // Уставка Vупр1, км/ч			0 – 255		1
	SpeedU2          byte   // Уставка Vупр2, км/ч			0 – 255		1
	TripTime         timeBU // Текущее время
}

// Событие – это структура данных установленного формата,
// которая отражает состояние одного из переменных параметров поездки.
type ivent struct {
	// id          uint
	len         int
	description string // для отладки
}

// все возможные события uint -- id
var mapIvent map[uint]ivent

// сначала нужно проверить хватит ли цельных регистраций для проверки скрипта
// можно создать отдельный мап с ид, которые занимают только байт (и не нужы?) их можно будет просто пропускать а таких много
// можно создать отдельный мап с ид, которые нужны

func createMapIvent() {

	mapIvent = make(map[uint]ivent)

	// 00h..3Bh	1	Время = Время + Идент.z + 1 (с)
	mapIvent[0x3C] = ivent{2, "Время + СС (с)"} // прибавить второй байт к текущему значению
	mapIvent[0x41] = ivent{1, "Скорость + 1 (км/ч)"}
	mapIvent[0x42] = ivent{1, "Скорость + 2 (км/ч)"}
	mapIvent[0x43] = ivent{1, "Скорость + 3 (км/ч)"}
	mapIvent[0x44] = ivent{1, "Скорость + 4 (км/ч)"}
	mapIvent[0x45] = ivent{1, "Скорость - 1 (км/ч)"}
	mapIvent[0x46] = ivent{1, "Скорость - 2 (км/ч)"}
	mapIvent[0x47] = ivent{1, "Скорость - 3 (км/ч)"}
	mapIvent[0x48] = ivent{1, "Скорость - 4 (км/ч)"}
	mapIvent[0x49] = ivent{2, "Скорость = СК (км/ч)"}       // до 256
	mapIvent[0x4A] = ivent{2, "Скорость = СК + 256 (км/ч)"} // больше 256
	mapIvent[0x51] = ivent{1, "Давление + 0,1 (кгс/см2)"}
	mapIvent[0x52] = ivent{1, "Давление + 0,2 (кгс/см2)"}
	mapIvent[0x53] = ivent{1, "Давление + 0,3 (кгс/см2)"}
	mapIvent[0x54] = ivent{1, "Давление - 0,1 (кгс/см2)"}
	mapIvent[0x55] = ivent{1, "Давление - 0,2 (кгс/см2)"}
	mapIvent[0x56] = ivent{1, "Давление - 0,3 (кгс/см2)"}
	mapIvent[0x57] = ivent{2, "Давление = Д (0,1 кгс/см2)"}
	mapIvent[0x58] = ivent{1, "Кран 395 = «отпуск»"}
	mapIvent[0x59] = ivent{1, "Кран 395 = «перекрыша»"}
	mapIvent[0x5A] = ivent{1, "Кран 395 = «торможение»"}
	mapIvent[0x5B] = ivent{1, "Кран 395 = «экстр. торможение»"}
	mapIvent[0x5C] = ivent{1, "Кран 395 = «перекрыша  САУТ»"}
	mapIvent[0x5D] = ivent{1, "Кран 395 = «торможение САУТ»"}
	mapIvent[0x5E] = ivent{2, "Кран 395 = КОД (неверный код)"} // ош
	mapIvent[0x60] = ivent{1, "AЛC = Нет огня"}
	mapIvent[0x61] = ivent{1, "AЛC = Зеленый"}
	mapIvent[0x62] = ivent{1, "AЛC = Желтый"}
	mapIvent[0x63] = ivent{1, "AЛC = Желтый с красным"}
	mapIvent[0x64] = ivent{1, "AЛC = Красный"}
	mapIvent[0x65] = ivent{1, "AЛC = Белый"}
	mapIvent[0x66] = ivent{1, "Белый, желтый с красным (УКБМ)"}
	mapIvent[0x67] = ivent{2, "Огни = КОД"}
	mapIvent[0x70] = ivent{2, "Ускорение = –УСК (0,01 м/с2)"}
	mapIvent[0x71] = ivent{1, "Направление = «Вперед»"}
	mapIvent[0x72] = ivent{1, "Направление = «Назад»"}
	mapIvent[0x73] = ivent{1, "ЭПК1 = «Выключен»"}
	mapIvent[0x74] = ivent{1, "ЭПК1 = «Включен»"}
	mapIvent[0x75] = ivent{1, "Ключ ЭПК = «Выключен»"}
	mapIvent[0x76] = ivent{1, "Ключ ЭПК = «Включен»"}
	mapIvent[0x77] = ivent{1, "Катушка ЭПК = «Выключена»"}
	mapIvent[0x78] = ivent{1, "Катушка ЭПК = «Включена»"}
	mapIvent[0x79] = ivent{1, "Разобщительный кран = «Нет давления»"}
	mapIvent[0x7A] = ivent{1, "Разобщительный кран = «Есть давление»"}
	mapIvent[0x7B] = ivent{1, "ЭПТ = «Выключен»"}
	mapIvent[0x7C] = ivent{1, "ЭПТ = «Включен»"}
	mapIvent[0x7D] = ivent{1, "Тумблер ДЗ = «С АЛС»"}
	mapIvent[0x7E] = ivent{1, "Тумблер ДЗ = «Без АЛС»"}
	mapIvent[0x80] = ivent{1, "Период кодирования = 1,6 с (время – 0 с)"}
	mapIvent[0x81] = ivent{1, "Период кодирования = 1,6 с (время – 1 с)"}
	mapIvent[0x82] = ivent{1, "Период кодирования = 1,6 с (время – 2 с)"}
	mapIvent[0x83] = ivent{1, "Период кодирования = 1,6 с (время – 3 с)"}
	mapIvent[0x84] = ivent{2, "Период кодирования = 1,6 с (время – СЕК)"} // второй байт время
	mapIvent[0x85] = ivent{1, "Период кодирования = 1,9 с (время – 0 с)"}
	mapIvent[0x86] = ivent{1, "Период кодирования = 1,9 с (время – 1 с)"}
	mapIvent[0x87] = ivent{1, "Период кодирования = 1,9 с (время – 2 с)"}
	mapIvent[0x88] = ivent{1, "Период кодирования = 1,9 с (время – 3 с)"}
	mapIvent[0x89] = ivent{2, "Период кодирования = 1,9 с (время – СЕК)"} // второй байт время
	mapIvent[0x8A] = ivent{1, "Отсутствие завершения периода"}            // ош
	mapIvent[0x90] = ivent{7, "Дополнительная регистрация"}               // ЧЧ ММ ПР (Время = «ЧЧ:ММ», Пробег = ПР(м))
	mapIvent[0x91] = ivent{1, "Ведущая голова = «Первая»"}
	mapIvent[0x92] = ivent{1, "Ведущая голова = «Вторая»"}
	mapIvent[0xA0] = ivent{1, "Код рельсовой цепи = «Нет кода»"}
	mapIvent[0xA1] = ivent{1, "Код рельсовой цепи = «Желтый с красным»"}
	mapIvent[0xA2] = ivent{1, "Код рельсовой цепи = «Желтый»"}
	mapIvent[0xA3] = ivent{1, "Код рельсовой цепи = «Зеленый» (или ошибка)"}
	mapIvent[0xA5] = ivent{1, "ЭПК2 = «Выключен»"}
	mapIvent[0xA6] = ivent{1, "ЭПК2 = «Включен»"}
	mapIvent[0xA7] = ivent{1, "Кнопка_помощника = «Отпущена»"}
	mapIvent[0xA8] = ivent{1, "Кнопка_помощника = «Нажата»"}
	mapIvent[0xA9] = ivent{1, "ТСКБМ = «Выключен»"}
	mapIvent[0xAA] = ivent{1, "ТСКБМ = «Включен»"}
	mapIvent[0xAB] = ivent{1, "САУТ = «Выключен»"}
	mapIvent[0xAC] = ivent{1, "САУТ = «Включен»"}
	mapIvent[0xAD] = ivent{3, "Сигналы пожарной безопасности"} //СПБ1К СПБ2К
	mapIvent[0xB0] = ivent{0xFF, "Сбои = «CБ1, …, СБN»"}       // переменная длинна! 2(N+1)
	mapIvent[0xB1] = ivent{5, "ГЛНС_коорд. = ГЛНС_коорд. + (Dφ, Dλ)"}
	mapIvent[0xB2] = ivent{9, "ГЛНС_коорд. = (φ , λ)"}
	mapIvent[0xB3] = ivent{1, "ГЛНС_коорд. = «не определена»"}
	mapIvent[0xC4] = ivent{9, "Ввод начальных параметров"} //  (Дата = Ч/М/Г, № поезда = НП, Таб. №
	mapIvent[0xC5] = ivent{3, "Коррекция времени (Время = «ЧЧ:ММ:00»)"}
	mapIvent[0xC8] = ivent{4, "Пробег (с начала поездки) = ПР (м)"}
	mapIvent[0xC9] = ivent{4, "Время = ЧЧ:ММ:CC"}
	mapIvent[0xCF] = ivent{21, "Единица_дискретности, Объем_топлива"} // ЕД0 ОТ0..ЕД3 ОТ3
	mapIvent[0xD0] = ivent{7, "Единица_энергии[НК] = ЕД"}
	mapIvent[0xD1] = ivent{2, "Давл.[НК] + 0,1 (кгс/см2)"}
	mapIvent[0xD2] = ivent{2, "Давл.[НК] + 0,2 (кгс/см2)"}
	mapIvent[0xD3] = ivent{2, "Давл.[НК] + 0,3 (кгс/см2)"}
	mapIvent[0xD4] = ivent{2, "Давл.[НК] - 0,1 (кгс/см2)"}
	mapIvent[0xD5] = ivent{2, "Давл.[НК] – 0,2 (кгс/см2)"}
	mapIvent[0xD6] = ivent{2, "Давл.[НК] – 0,3 (кгс/см2)"}
	mapIvent[0xD7] = ivent{3, "Давл.[НК] = Д (0,1 кгс/см2)"}
	mapIvent[0xD8] = ivent{1, "Скорость + 0,5 (км/ч)"}
	mapIvent[0xD9] = ivent{1, "Скорость + 1,5 (км/ч)"}
	mapIvent[0xDA] = ivent{1, "Скорость - 0,5 (км/ч)"}
	mapIvent[0xDB] = ivent{1, "Скорость - 1,5 (км/ч)"}
	mapIvent[0xDC] = ivent{2, "Скорость = СК + 0,5 (км/ч)"}
	mapIvent[0xDD] = ivent{2, "Скорость = СК + 256,5 (км/ч)"}
	mapIvent[0xE0] = ivent{1, "Объем топлива -1 ед. дискр."}
	mapIvent[0xE1] = ivent{1, "Объем топлива - 2 ед. дискр."}
	mapIvent[0xE2] = ivent{2, "Объем топлива - ОТ ед. дискр."}
	mapIvent[0xE9] = ivent{1, "Объем топлива = Объем топлива + 1 ед. дискр."}
	mapIvent[0xEA] = ivent{1, "Объем топлива + 2 ед. дискр."}
	mapIvent[0xEB] = ivent{2, "Объем топлива + ОТ ед. дискр."}
	mapIvent[0xEE] = ivent{3, "Положение_контроллера = КОД"}
	mapIvent[0xF0] = ivent{2, "Темп. топлива = Т (ºС)"}
	mapIvent[0xF1] = ivent{2, "Плотн. топлива = 800 + Р (кг/м3)"}
	mapIvent[0xF2] = ivent{3, "Темп. топлива = Т (ºС) / Плотн. топлива = 800 + P (кг/м3)"}
	mapIvent[0xF3] = ivent{1, "ТЯГА = «Выключена»"}
	mapIvent[0xF4] = ivent{1, "ТЯГА = «Включена»"}
	mapIvent[0xF5] = ivent{2, "Номер бака = НБ"}
	mapIvent[0xF6] = ivent{3, "Масса топлива = М (кг)"}
	mapIvent[0xF8] = ivent{0xFF, "Дополнительные параметры"} // переменная длинна
	mapIvent[0xF9] = ivent{0xFF, "Дополнительные данные"}    // переменная длинна
	mapIvent[0xFF] = ivent{1, "0xFF - пустой байт, пропустить"}
}

// переменные для отслеживания значения, если явных регистраций не хватает
var gTime time.Time
var gPress float64
var gAnalogSignal1 float64
var gAnalogSignal2 float64
var gAnalogSignal3 float64
var gSpeed float64

// все явные регистрации
var fSpeedSlice []float64
var fPressSlice1 []float64 // у 3ПС: fPressSlice1 = "Давление", 2 = "аналог сигн 1", и 3 = "аналог сигнал 2"
var fPressSlice2 []float64
var fPressSlice3 []float64
var uMilageSlice []uint64
var fAccelSlice []float64
var mapALS map[uint]ivent
var uPeriodSlice []uint   // период кодирования
var uCodesRCSlice []uint  // коды рельсовой цепи
var uCrane395Slice []uint // позиция крана 395 (id)
var uFreqDGUSlice []uint64
var mapBUSM map[uint]ivent  // сигналы БУС-М (срез содержит ИД + описание)
var uFireAlarmSlice1 []byte // Сигналы пожарной безопасности первой кабины локомотива (срез содержит байты - каждый бит уст\сброс ПЖ)
var uFireAlarmSlice2 []byte // Сигналы пожарной безопасности второй кабины локомотива (срез содержит байты - каждый бит уст\сброс ПЖ)
var uFaults []uint16        // Сбои (срез содержит коды сбоев)
var uAddSignal []uint8      // состояния двоичного входа Дополнительный сигнал

func initAllMaps() {
	mapALS = make(map[uint]ivent)
	mapBUSM = make(map[uint]ivent)
}

// добавить проверки на длинну массива больше1
// добавить не явные регистрации
// выводы ош из мап
