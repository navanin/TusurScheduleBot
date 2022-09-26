package main

import (
	"database/sql"
	"fmt"
	"github.com/PuloV/ics-golang"
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

	// Эксешпшн на неправильную группу - ToDo
	return ""
}

func removeFiles(fileName string) {

	// Функция removeFiles() принимает название файла в качестве аргумента,
	// и с помощью модуля os удаляет файл, если он существует.

	if _, err := os.Stat(fileName); err == nil {
		os.Remove(fileName)
	}

	log2file("Removed file "+fileName, nil)
}

func getSchedule(groupNumber string) {

	// Функция, созданная для получения расписания с сайта в формате в .ics (iCalendar), и запись его в файл,
	// для последующего парсинга из него.

	// Вызов функции для удаления предыдущего файла с расписанием выбранной группы. Костыль, но расписание в ТУСУРе непостоянное.
	removeFiles("./groups/" + groupNumber + ".ics")

	// Создание файла *номер_группы*.ics для записи в него полученного расписания.
	file, _ := os.Create("./groups/" + groupNumber + ".ics")
	log2file("Created file ./groups/"+groupNumber+".ics", nil)

	// Генерация ссылки для получения расписания. Вызов функции getFaculty() подставляет в ссылку факультет, а переменная groupNumber указывает номер учебной группы.
	url := "https://timetable.tusur.ru/faculties/" + getFaculty(groupNumber) + "/groups/" + groupNumber + ".ics"

	// GET-запрос на ссылку для получения расписания
	resp, err := http.Get(url)

	if err != nil {
		log2file("Couldn't get timetable of "+groupNumber+".", err)
		return
	}

	// Копирование полученного на запрос GET, ответа в файл
	defer resp.Body.Close()
	defer file.Close()
	_, err = io.Copy(file, resp.Body)

	log2file("Got timetable of "+groupNumber+".", nil)
}

func parseSchedule(groupNumber string, date string) {

	// Функция parseSchedule() отвечает за поиск пар на конкретный день в файле расписания и последующую передачу найденных
	// пар в массив lessons, который будет хранить информацию о занятиях до конца работы с сообщением.

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

	log2file("Created parser on "+groupNumber+".ics", nil)

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
	message += fmt.Sprintf("Расписание группы %s на %s.\n\n", groupNumber, fmtDate.Format("02.01.2006"))

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
	log2file("Formed message for "+groupNumber+" .", nil)
	return message
}

func getBinding(db *sql.DB, conversationId int) (bool, string) {

	// Функция для проверки наличия ассоциации группы с чатом ВК.
	// Происходит поиск в таблице по ID чата ВК, если запись имеется, то возвращается положительный результат и номер группы,
	// иначе, отрицательный рез-тат и пустая строка, вместо номера группы.

	var dbAnswer string
	err := db.QueryRow("select groupNumber from binds where groupid = ?", conversationId).Scan(&dbAnswer)
	if err != nil {
		log2file("ERROR on getBinding() with"+string(rune(conversationId))+".", err)
	}
	log2file(fmt.Sprintf("getBinding() with %d .", conversationId), nil)
	if dbAnswer == "" {
		// Если dbAnswer пуста, значит, ассоциация пока не была создана.
		return false, ""
	} else {
		// Иначе, ассоциация имеется.
		return true, dbAnswer
	}
}

func setBinding(db *sql.DB, conversationId int, groupNumber string) bool {

	// Функция для установки ассоциации группы с чатом ВК.
	// Таблица с ассоциациями дополняется новой парой "ID чата - номер группы"

	sqlStatement := "insert into binds(groupId, groupNumber) values (?, ?);"
	stmt, _ := db.Prepare(sqlStatement)
	_, err := stmt.Exec(conversationId, groupNumber)
	stmt.Close()

	// Проверяется наличие информации в переменной err.
	if err != nil {
		// Если err не пуста, значит что-то пошло не так, и пара не была добавлена.
		log2file(fmt.Sprintf("ERROR on setBinding() with %d and %s.", conversationId, groupNumber), nil)

		return false
	} else {
		// Иначе, пара добавлена, функция возвращает положительный ответ.
		log2file(fmt.Sprintf("setBinding() with %d and %s.", conversationId, groupNumber), nil)
		return true
	}
}

func rmBinding(db *sql.DB, conversationId int) bool {

	// Функция для удаления ассоциации группы с чатом ВК.
	// В таблице ассоциаций проходит поиск по ID чата, если таковой найден - строка удаляется.

	sqlStatement := "DELETE FROM binds WHERE groupId = ?;"
	stmt, _ := db.Prepare(sqlStatement)
	_, err := stmt.Exec(conversationId)
	stmt.Close()

	// Проверяется наличие информации в переменной err.
	if err != nil {
		// Если err не пуста, значит что-то пошло не так, и пара не была удалена.
		log2file(fmt.Sprintf("ERROR on rmBinding() with %d.", conversationId), nil)
		return false
	} else {
		// Иначе, пара удалена, функция возвращает положительный ответ.
		log2file(fmt.Sprintf("rmBinding() with %d.", conversationId), nil)
		return true
	}
}

func getBindingsInfo(db *sql.DB) string {

	var message string
	var groupId string
	var groupNumber string
	var counter = 0

	rows, err := db.Query("select groupID, groupNumber from binds")

	message = "Актуальные ассоциации в БД:\n"

	defer rows.Close()
	for rows.Next() {

		counter++

		err = rows.Scan(&groupId, &groupNumber)
		if err != nil {
			log2file(fmt.Sprintf("ERROR on getBindingsInfo()"), nil)
		}
		message += fmt.Sprintf("%d. Чат %s - группа %s\n", counter, groupId, groupNumber)
	}
	return message
}

func log2file(str string, err error) {
	f, _ := os.OpenFile("bot.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	defer f.Close()
	log := fmt.Sprintf("%s %s", time.Now().Format("15:04:05 02/01/2006"), str)
	if err != nil {
		log += fmt.Sprint(err)
	}
	f.WriteString(log + "\n")
}
