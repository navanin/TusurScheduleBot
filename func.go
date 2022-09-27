package main

import (
	"fmt"
	"github.com/PuloV/ics-golang"
	"github.com/essentialkaos/translit/v2"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Массив пар, типа ics.Event - событие календаря
var lessons = make([]ics.Event, 0)

func getFaculty(groupNumber string) string {

	// Функция, определяющая, к какому факультету относится группа, согласно первой цифре номера группы.
	// 1 - РТФ,		4 - ФСУ,	7 - ФБ,
	// 2 - РКФ, 	5 - ФВС,	8 - ЭФ
	// 3 - ФЭТ,		6 -  ГФ,
	// 0 - исключение, на эту цифру начинается и ЮФ(9) и ФИТ, поэтому определение идет по второй цифре.

	switch groupNumber[0] {
	case '1':
		return "rtf"
	case '2':
		return "rkf"
	case '3':
		return "fet"
	case '4':
		return "fsu"
	case '5':
		return "fvs"
	case '6':
		return "gf"
	case '7':
		return "fb"
	case '8':
		return "ef"
	case '0':
		if groupNumber[1] == '9' {
			return "yuf"
		} else {
			return "fit"
		}
	}

	return ""
}

func removeFiles(fileName string) {

	// Функция removeFiles() принимает название файла в качестве аргумента,
	// и с помощью модуля os удаляет файл, если он существует.

	if _, err := os.Stat(fileName); err == nil {
		os.Remove(fileName)
	}
}

func getSchedule(groupNumber string) {

	// Функция, созданная для получения расписания с сайта в формате в .ics (iCalendar), и запись его в файл,
	// для последующего парсинга из него.

	// Вызов функции для удаления предыдущего файла с расписанием выбранной группы. Костыль, но расписание в ТУСУРе непостоянное.

	removeFiles("./groups/" + groupNumber + ".ics")

	// Создание файла *номер_группы*.ics для записи в него полученного расписания.
	file, _ := os.Create("./groups/" + groupNumber + ".ics")

	// Генерация ссылки для получения расписания. Вызов функции getFaculty() подставляет в ссылку факультет, а переменная groupNumber указывает номер учебной группы.
	url := "https://timetable.tusur.ru/faculties/" + getFaculty(groupNumber) + "/groups/" + groupNumber + ".ics"

	// GET-запрос на ссылку для получения расписания
	resp, _ := http.Get(url)

	// Копирование полученного на запрос GET, ответа в файл
	defer resp.Body.Close()
	defer file.Close()
	_, _ = io.Copy(file, resp.Body)

}

func parseSchedule(groupNumber string, date string) {

	// Функция parseSchedule() отвечает за поиск пар на конкретный день в файле расписания и последующую передачу найденных
	// пар в массив lessons, который будет хранить информацию о занятиях до конца работы с сообщением.

	groupNumber = translit.EncodeToICAO(groupNumber)

	// Функция getSchedule() вызывается для получения максимально актуального расписания группы.
	getSchedule(groupNumber)

	// Массив событий календаря, в котором будут храниться занятия
	lessons = make([]ics.Event, 0)

	// Создание нового парсера календаря, указание параметров и имени файла, подлежащего парсингу.
	parser := ics.New()
	ics.FilePath = "./"
	ics.DeleteTempFiles = false
	ics.RepeatRuleApply = true
	inputChan := parser.GetInputChan()
	inputChan <- "./groups/" + groupNumber + ".ics"

	// parser.Wait() используется для ожидания завершения работы парсера. Результаты парсинга записываются в переменную cal, нулевой индекс.
	parser.Wait()
	cal, _ := parser.GetCalendars()

	// Перебор событий, полученных из календаря
	for _, e := range cal[0].GetEvents() {
		// Проверка, соответствует ли дата начала события, дате указанной при вызове функции.
		if e.GetStart().Format(ics.IcsFormatWholeDay) == date {

			// При соблюдении условия, событие добавляется в массив пар.
			lessons = append(lessons, e)
		} else {

			// Иначе, просматривается следующее событие.
			continue
		}
	}
	// После перебора событий и добавления их в массив,
	// вызывается функция sortArray() для его сортировки
	sortArray()
}

func sortArray() {

	// Сортировка пузырьком.
	// Сортируется по времени начала события. Оказывается GO умеет сравнивать строки.
	// "10:00" > "09:00" == true

	for i := 0; i < len(lessons)-1; i++ {
		for j := 0; j < len(lessons)-i-1; j++ {
			if lessons[j].GetStart().Format("15:04") > lessons[j+1].GetStart().Format("15:04") {
				lessons[j], lessons[j+1] = lessons[j+1], lessons[j]
			}
		}
	}
}

func formMessage(groupNumber string, date string) string {

	// Функция formMessage() отвечает за формирование конечного сообщения.
	// В качестве аргументов получает дату и номер группы.
	// По скольку массив с парами является глобальным, его не нужно передавать в функцию.

	var message = ""
	var lessonType string // Лекция/Практика/Сам.раб/Лаб.раб - получает из поля события "Description".
	var teacher string    // Фамилия И.О. преподавателя - получает из поля события "Description".
	var classroom string  // номер аудитории - получает из поля события "Location" (не путать с "Geo").

	// Получение даты из аргументов.
	var fmtDate, _ = time.Parse("20060102", date)

	// Формирование шапки сообщения.
	message += fmt.Sprintf("Расписание группы %s на %s (%s).\nВсего занятий - %d.\n\n", groupNumber, fmtDate.Format("02.01.2006"), getRuWeekDay(fmtDate), len(lessons))

	if len(lessons) == 0 {
		message += "Занятий нет - выходные 🥳"
	}

	// Цикличный перебор массива пар, для формирования сообщения с расписанием.
	for i := 0; i <= len(lessons)-1; i++ {

		// !Костыль!
		// Так как тип пары и препод хранятся в одном и том же поле, приходиться
		// прибегать к подобному делению переменной.
		// Аудитории разделяются слешами, из-за этого их убираю с помощью strings.ReplaceAll().
		descriptionSplit := strings.Split(lessons[i].GetDescription(), "\\, ")
		lessonType = descriptionSplit[0]
		teacher = descriptionSplit[1]
		classroom = strings.ReplaceAll(lessons[i].GetLocation(), "\\", "")

		// На каждой итерации цикла, к финальному сообщению добавляется следующая пара.
		message += fmt.Sprintf("📖 %s (%s)\n ‍👨 Преподаватель: %s\n 🏠 Аудитория: %s\n 🕛 Время: %s-%s\n\n",
			lessons[i].GetSummary(), lessonType, teacher, classroom, //lessons[i].GetLocation(),
			lessons[i].GetStart().Format("15:04"), lessons[i].GetEnd().Format("15:04"))
	}
	return message
}
