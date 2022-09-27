package main

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/api/params"
	"github.com/SevereCloud/vksdk/v2/events"
	"github.com/SevereCloud/vksdk/v2/longpoll-bot"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"regexp"
	"strings"
	"time"
)

func main() {

	// dev

	// Подключение к БД sqlite3
	db, err := sql.Open("sqlite3", "./sqlite.db")

	// Подключение к API VK с помощью токена, и получение группы, от которой был получен токен.
	vk := api.NewVK(TOKEN)
	group, _ := vk.GroupsGetByID(nil)

	go cronSending(db, vk)

	// Создание нового lonpoll'а для обработки событий
	lp, _ := longpoll.NewLongPoll(vk, group[0].ID)

	// Функция, обрабатывающая новое событие получения нового сообщения.
	lp.MessageNew(func(_ context.Context, obj events.MessageNewObject) {

		var message = ""
		b := params.NewMessagesSendBuilder()
		b.RandomID(0)
		b.PeerID(obj.Message.PeerID)

		// Перевод сообщения в нижний регистр для последующего поиска в нем.
		text := strings.ToLower(obj.Message.Text)

		// Блок сообщений-команд.

		if strings.Contains(text, "/help") {

			// Если сообщение содержит текст "/help", то в качестве ответа будет отправлена переменная helpMsg (messages.go),
			// содержащее список команд и полезной информации.

			// Сборка сообщения-ответа.
			b.Message(helpMsg)
			vk.MessagesSend(b.Params)
			return
		}

		if strings.Contains(text, "/bind") {

			// Если сообщение содержит текст "/bind", необходимо обнаружить номер группы, отправленный в сообщении,
			// проверить наличие существующей ассоциации и, при её отсутствии, создать с ней новую.

			var groupNumber string

			// Номер группы в сообщении обнаруживается с помощью регулярного выражения.
			re := regexp.MustCompile(`(\d\w\d)(\-\w{0,2})?`)
			groupNumber = re.FindString(text)

			bindFlag, bindGroup := getBinding(db, obj.Message.PeerID)

			if groupNumber != "" {
				if !bindFlag {
					if setBinding(db, obj.Message.PeerID, groupNumber) {
						// Если результат положительный, обработка сообщения заканчивается и задается финальное сообщение.
						message = fmt.Sprintf(successfulBindMsg, groupNumber)
					} else {
						// Если результат отрицательный, обработка сообщения заканчивается ошибкой и задается финальное сообщение о ней.
						message = unhandledErrMsg
					}
				} else {
					rmBinding(db, obj.Message.PeerID)
					setBinding(db, obj.Message.PeerID, groupNumber)
					message = fmt.Sprintf(successfulRebindMsg, bindGroup, groupNumber)
				}
			} else {
				// Если синтаксис команды неправильный, обработка сообщения заканчивается ошибкой и задается финальное сообщение о ней.
				message = fmt.Sprintf(infoBindMsg, bindGroup)
			}

			// Сборка сообщения-ответа.
			b.Message(message)
			vk.MessagesSend(b.Params)
			return
		}

		if strings.Contains(text, "/unbind") {

			// Если сообщение содержит текст "/unbind", необходимо обнаружить номер группы, отправленный в сообщении,
			// проверить наличие существующей с ней ассоциации и, при её наличии, удалить ее.

			isBound, _ := getBinding(db, obj.Message.PeerID)

			if !isBound {
				message = noBindMsg
			}
			// Для удаления ассоциации вызывается функция rmBinding().
			if rmBinding(db, obj.Message.PeerID) {

				// В случае положительного ответа от функции, обработка сообщения заканчивается и задается финальное сообщение.
				message = "Ассоциация удалена."
			} else {
				// Иначе, обработка сообщения заканчивается ошибкой и задается финальное сообщение о ней.
				message = unhandledErrMsg
			}

			// Сборка сообщения-ответа.
			b.Message(message)
			vk.MessagesSend(b.Params)
			return
		}

		// Блок служебных команд

		if strings.Contains(text, "/db") {

			// Если сообщение содержит текст "/db", необходимо отправить все существующие ассоциации чатов с группами в качестве ответа.

			// Так как функция служебная, необходимо проверять, от кого приходит сообщение.
			// Если сообщение пришло не от меня, то в качестве ответа отправляется сообщение о нехватке доступа.
			if obj.Message.PeerID != 366661090 {
				b.Message(noAccess)
			} else {
				// Иначе отправляется информация об ассоциациях.
				b.Message(getBindingsInfo(db))
			}
			vk.MessagesSend(b.Params)
			return
		}

		if strings.Contains(text, "/upd ") {

			// Если сообщение содержит текст "/udp", то в качестве ответа будет отправлена переменная helpMsg (messages.go),
			// содержащее список команд и полезной информации.

			if obj.Message.PeerID != 366661090 {
				b.Message(noAccess)
			} else {
				msg := strings.ReplaceAll(obj.Message.Text, "/upd", "")
				b.PeerID(366661090)
				b.Message(sendUpdMessage(db, vk, msg))
			}
			vk.MessagesSend(b.Params)
			// Сборка сообщения-ответа.
			return
		}

		// Блок расписания.

		if strings.Contains(text, "расписос на завтра") {

			// "Расписос на завтра" подразумевает все то же самое, что и "расписос", но на дату завтрашнего дня.

			var date string
			var tomorrow = time.Now().AddDate(0, 0, 1)
			var groupNumber string
			var bindFlag bool

			re := regexp.MustCompile(`(\d\w\d)(\-\w{0,2})?`)
			groupNumber = re.FindString(text)

			re = regexp.MustCompile(`\d\d\.\d\d`)
			date = re.FindString(text)

			if date != "" {
				date = date + ".2022"
			} else {
				if isSunday(tomorrow) {
					date = tomorrow.AddDate(0, 0, 1).Format("20060102")
					message += exTommorowIsSunday
				} else {
					// Иначе, используется сегодняшняя дата
					date = tomorrow.Format("20060102")
				}
			}

			if groupNumber == "" {
				bindFlag, groupNumber = getBinding(db, obj.Message.PeerID)
				if !bindFlag {
					message = raspisosTommorowUsage
					b.Message(message)
					vk.MessagesSend(b.Params)
					return
				}
			}
			parseSchedule(groupNumber, date)
			message = formMessage(groupNumber, date)

			// Собираем сообщение-ответ
			b.Message(message)
			vk.MessagesSend(b.Params)
			return
		}

		if strings.Contains(obj.Message.Text, "расписос") {

			var date string
			var today = time.Now()
			var groupNumber string
			var bindFlag bool

			re := regexp.MustCompile(`(\d\w\d)(\-\w{0,2})?`)
			groupNumber = re.FindString(text)

			re = regexp.MustCompile(`\d\d\.\d\d`)
			date = re.FindString(text)

			if date != "" {
				date = date + ".2022"
				splitDate := strings.Split(date, ".")
				date = fmt.Sprintf("%s%s%s", splitDate[2], splitDate[1], splitDate[0])
			} else {
				if isSunday(today) {
					date = today.AddDate(0, 0, 1).Format("20060102")
					message += exTodayIsSunday
				} else {
					// Иначе, используется сегодняшняя дата
					date = today.Format("20060102")
				}
			}

			if groupNumber == "" {
				bindFlag, groupNumber = getBinding(db, obj.Message.PeerID)
				if !bindFlag {
					message = raspisosUsage
					b.Message(message)
					vk.MessagesSend(b.Params)
					return
				}
			}

			parseSchedule(groupNumber, date)
			message = formMessage(groupNumber, date)

			// Собираем сообщение-ответ
			b.Message(message)
			vk.MessagesSend(b.Params)
			return
		}
	})

	// Запуск lp-хендлера
	err = lp.Run()
	if err != nil {
		log.Fatal(err)
	}

}
