package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/api/params"
	"github.com/stephenafamo/kronika"
	"time"
)

func isSunday(date time.Time) bool {
	if date.Weekday().String() == "Sunday" {
		return true
	} else {
		return false
	}
}

func getRuWeekDay(date time.Time) string {
	switch date.Weekday() {
	case 1:
		return "Понедельник"
	case 2:
		return "Вторник"
	case 3:
		return "Среда"
	case 4:
		return "Четверг"
	case 5:
		return "Пятница"
	case 6:
		return "Суббота"
	case 7:
		return "Воскресенье"
	}
	return ""
}

func sendUpdMessage(db *sql.DB, vk *api.VK, message string) string {

	// Функция для рассылки произвольного сообщения по всем беседам, в которых были созданы ассоциации.

	var groupId = 0
	var response = fmt.Sprintf("Сообщение: \n\"%s\"\n\nБыло отправлено в: ", message)
	// Запрос к БД на получение всех ID ассоциированных чатов
	rows, _ := db.Query("select groupID from binds")

	defer rows.Close()
	for rows.Next() {

		// Получение ID бесед
		rows.Scan(&groupId)

		println(groupId)

		// Отправка переданного в функцию сообщения в беседу с соответствующим groupId
		b := params.NewMessagesSendBuilder()
		b.RandomID(0)
		b.PeerID(groupId)
		b.Message(message)
		vk.MessagesSend(b.Params)
		response += fmt.Sprintf("%d", groupId)
	}
	return response
}

func cronSending(db *sql.DB, vk *api.VK) {

	// Функция cronSending() отвечает за запланированную отправку расписания утром (в 8:00) и вечером (в 20:00).
	// Расписание отправляемое утром - на сегодняшний день, вечером - на завтрашний.
	// Используется пакет kronika (github.com/stephenafamo/kronika).

	var message = ""

	// Благодаря context.Background(), функция не имеет ни дедлайнов, ни переменных, и выполняется в бэкграунде.
	ctx := context.Background()

	// В переменной start хранится время начала выполнения kronika-действия. 20:00
	// Переменная interval указывает интервал выполнения действия. 12 часов
	start, _ := time.Parse("2006-01-02 15:04:05", "2022-01-01 20:00:00")
	interval := time.Hour * 12 //  8:00 am and 20:00 pm

	// Начало kronika-действия. В kronika.Every() передается контекст, интервал и время начала.
	for range kronika.Every(ctx, start, interval) {

		var groupId int
		var groupNumber string
		var date string

		var today = time.Now()
		var tommorow = time.Now().AddDate(0, 0, 1)

		// В переменную rows записывается ответ, полученной об БД после запроса.
		// Цель запроса - получить все данные обо всех ассоциациях
		rows, _ := db.Query("select groupID, groupNumber from binds")

		defer rows.Close()
		for rows.Next() {

			// На каждый полученный ответ выполняется следующие действия:
			// 1. Ответ записывается в переменные groupID и groupNumber;
			// 2. Определяется время выполнение скрипта:
			// 	2.1 Если время 08:00, то отправляется расписание на сегодняшний день;
			// 	2.2 Если время 20:00, то отправляется расписание на завтрашний день.
			rows.Scan(&groupId, &groupNumber)

			if time.Now().Hour() == 8 {

				// Проверка на выходной день
				if isSunday(today) {
					// Если сегодня воскресенье, то к дате прибавляется один день
					date = today.AddDate(0, 0, 1).Format("20060102")
					message += "Сегодня воскресенье, но вот расписание на понедельник: \n"
				} else {
					// Иначе, используется сегодняшняя дата
					date = today.Format("20060102")
				}

				parseSchedule(groupNumber, date)
				message = formMessage(groupNumber, date)

				b := params.NewMessagesSendBuilder()
				b.Message(message)
				b.RandomID(0)
				b.PeerID(groupId) // groupId
				vk.MessagesSend(b.Params)

			} else if time.Now().Hour() == 20 {

				// Проверка на выходной день
				if isSunday(tommorow) {
					// Если завтра воскресенье, то к дате прибавляется два дня, вместо одного
					date = tommorow.AddDate(0, 0, 1).Format("20060102")
					message += "Завтра воскресенье, но вот расписание на понедельник: \n"
				} else {
					// Иначе, используется завтрашняя дата
					date = tommorow.Format("20060102")
				}

				parseSchedule(groupNumber, date)
				message = formMessage(groupNumber, date)

				b := params.NewMessagesSendBuilder()
				b.Message(message)
				b.RandomID(0)
				b.PeerID(groupId) // groupId
				vk.MessagesSend(b.Params)
			}
			// Очистка сообщения после каждой итерации.
			message = ""
		}
	}
}
