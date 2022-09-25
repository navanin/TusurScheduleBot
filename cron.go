package main

import (
	"context"
	"database/sql"
	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/api/params"
	"github.com/stephenafamo/kronika"
	"log"
	"time"
)

// Очень экспериментальный файлик, который в последующем рефакторинге должен уехать в func.go
// Необходимо решить и сделать проблему с тем, что функция дублирует код main.go

func cronSending(db *sql.DB, vk *api.VK) {

	// Функция cronSending() отвечает за запланированную отправку расписания утром (в 8:00) и вечером (в 20:00).
	// Расписание отправляемое утром - на сегодняшний день, вечером - на завтрашний.
	// Используется пакет kronika (github.com/stephenafamo/kronika).

	// Благодаря context.Background(), функция не имеет ни дедлайнов, ни переменных, и выполняется в бэкграунде.
	ctx := context.Background()

	// В переменной start хранится время начала выполнения kronika-действия. 20:00
	// Переменная interval указывает интервал выполнения действия. 12 часов
	start, _ := time.Parse("2006-01-02 15:04:05", "2022-01-01 20:00:00")
	interval := time.Hour * 12 //  8:00 am and 20:00 pm

	// Начало kronika-действия. В kronika.Every() передается контекст, интервал и время начала.
	for _ = range kronika.Every(ctx, start, interval) {

		var groupId int
		var groupNumber string
		var date string

		// В переменную rows записывается ответ, полученной об БД после запроса.
		// Цель запроса - получить все данные обо всех ассоциациях
		rows, err := db.Query("select groupID, groupNumber from binds")
		if err != nil {
			log.Print("DB(cron-get) ERROR: ", err)
		}

		defer rows.Close()
		for rows.Next() {

			// На каждый полученный ответ выполняется следующие действия:
			// 1. Ответ записывается в переменные groupID и groupNumber;
			// 2. Определяется время выполнение скрипта:
			// 	2.1 Если время 08:00, то отправляется расписание на сегодняшний день;
			// 	2.2 Если время 20:00, то отправляется расписание на завтрашний день.
			err = rows.Scan(&groupId, &groupNumber)
			if err != nil {
				log.Print("DB(cron-query-get) ERROR: ", err)
			}

			if time.Now().Hour() == 8 {

				// Проверка на выходной день
				if time.Now().Weekday().String() == "Sunday" {
					// Если сегодня воскресенье, то к дате прибавляется один день
					date = time.Now().AddDate(0, 0, 1).Format("20060102")
					message += "Сегодня воскресенье, но вот расписание на понедельник: \n"
				} else {
					// Иначе, используется сегодняшняя дата
					date = time.Now().Format("20060102")
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
				if time.Now().AddDate(0, 0, 1).Weekday().String() == "Sunday" {
					// Если завтра воскресенье, то к дате прибавляется два дня, вместо одного
					date = time.Now().AddDate(0, 0, 1).Format("20060102")
					message += "Завтра воскресенье, но вот расписание на понедельник: \n"
				} else {
					// Иначе, используется завтрашняя дата
					date = time.Now().AddDate(0, 0, 1).Format("20060102")
				}

				parseSchedule(groupNumber, date)
				message = formMessage(groupNumber, date)

				b := params.NewMessagesSendBuilder()
				b.Message(message)
				b.RandomID(0)
				b.PeerID(groupId) // groupId
				vk.MessagesSend(b.Params)
			} else {
				// Если действие выполняется не в 08 и не в 20, логируется ошибка, связанная со временем.
				log.Print("CRON ERROR: Какие-то ошибки со временем.")
			}
			// Очистка сообщения после каждой итерации.
			message = ""
		}
	}
}
