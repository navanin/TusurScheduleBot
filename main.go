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

	// Подключение к БД sqlite3
	db, err := sql.Open("sqlite3", "./sqlite.db")
	if err != nil {
		log.Print("DB(open) ERROR: ", err)
	}

	// Подключение к API VK с помощью токена, и получение группы, от которой был получен токен.
	vk := api.NewVK(TOKEN)
	group, _ := vk.GroupsGetByID(nil)

	// Создание нового lonpoll'а для обработки событий
	lp, _ := longpoll.NewLongPoll(vk, group[0].ID)

	// Функция, обрабатывающая новое событие получения нового сообщения.
	lp.MessageNew(func(_ context.Context, obj events.MessageNewObject) {

		// Перевод сообщения в нижний регистр для последующего поиска в нем.
		obj.Message.Text = strings.ToLower(obj.Message.Text)

		// Блок сообщений-команд.

		if strings.Contains(obj.Message.Text, "/help") {

			// Если сообщение содержит текст "/help", то в качестве ответа будет отправлена переменная helpMsg (messages.go),
			// содержащее список команд и полезной информации.

			// Сборка сообщения-ответа.
			b := params.NewMessagesSendBuilder()
			b.Message(helpMsg)
			b.RandomID(0)
			b.PeerID(obj.Message.PeerID)
			vk.MessagesSend(b.Params)
		}

		if strings.Contains(obj.Message.Text, "/bind") {

			// Если сообщение содержит текст "/bind", необходимо обнаружить номер группы, отправленный в сообщении,
			// проверить наличие существующей ассоциации и, при её отсутствии, создать с ней новую.

			// Номер группы в сообщении обнаруживается с помощью регулярного выражения.
			// К слову, из-за этого, группы с буквами в номере и не обнаруживаются =)
			haveNumber, _ := regexp.MatchString("/bind (\\d[А-яA-z0-9]\\d)(\\-[А-яA-z0-9]{0,2})?", obj.Message.Text)

			if haveNumber {

				// Если в сообщении обнаружен номер группы, в таком случае, он вычленяется из сообщения и передается
				// в функцию getBinding(), для проверки на наличие ассоциации.
				re := regexp.MustCompile(`(\d[А-яA-z0-9]\d)(\-[А-яA-z0-9]{0,2})?`)
				bindingGroupNumber := re.FindString(obj.Message.Text)
				bindFlag, _ := getBinding(db, obj.Message.PeerID)

				if !bindFlag {

					// Если функция getBinding() возвращает отрицательный результат, т.е. ассоциации не существует,
					// вызывается функция setBinding() для её создания.
					if setBinding(db, obj.Message.PeerID, bindingGroupNumber) {

						// Если результат положительный, обработка сообщения заканчивается и задается финальное сообщение.
						message = fmt.Sprintf("Теперь ваша группа автоматически будет получать расписание группы %s.\nДля получения подробной информации введите /help.", bindingGroupNumber)
					} else {
						// Если результат отрицательный, обработка сообщения заканчивается ошибкой и задается финальное сообщение о ней.
						message = "Что-то пошло не так. Уведомите об этом автора бота.\nДля получения подробной информации введите /help."
					}
				}
			} else {
				// Если синтаксис команды неправильный, обработка сообщения заканчивается ошибкой и задается финальное сообщение о ней.
				message = "Использование команды: /bind *номер_группы*.\nДля получения подробной информации введите /help."
			}

			// Сборка сообщения-ответа.
			b := params.NewMessagesSendBuilder()
			b.Message(message)
			b.RandomID(0)
			b.PeerID(obj.Message.PeerID)
			vk.MessagesSend(b.Params)
		}

		if strings.Contains(obj.Message.Text, "/unbind") {

			// Если сообщение содержит текст "/unbind", необходимо обнаружить номер группы, отправленный в сообщении,
			// проверить наличие существующей с ней ассоциации и, при её наличии, удалить ее.

			// Для удаления ассоциации вызывается функция rmBinding().
			if rmBinding(db, obj.Message.PeerID) {

				// В случае положительного ответа от функции, обработка сообщения заканчивается и задается финальное сообщение.
				message = "Ассоциация удалена."
			} else {
				// Иначе, обработка сообщения заканчивается ошибкой и задается финальное сообщение о ней.
				message = "Что-то пошло не так, либо ассоциации не существует.\n Если вы уверены, что это ошибка - уведомите об этом автора бота.\nДля получения подробной информации введите /help."
			}

			// Сборка сообщения-ответа.
			b := params.NewMessagesSendBuilder()
			b.Message(message)
			b.RandomID(0)
			b.PeerID(obj.Message.PeerID)
			vk.MessagesSend(b.Params)
		}

		// Блок расписания.

		if strings.Contains(obj.Message.Text, "расписос на завтра") {

			// "Расписос на завтра" подразумевает все то же самое, что и "расписос", но на дату завтрашнего дня.

			var date string

			// Проверка на выходной день
			if time.Now().Weekday().String() == "Sunday" {
				// Если завтра воскресенье, то прибавляется два дня, вместо одного
				date = time.Now().AddDate(0, 0, 2).Format("20060102")
				message += "Завтра воскресенье, но вот расписание на понедельник: \n"
			} else {
				// Иначе, прибаляется один день
				date = time.Now().AddDate(0, 0, 1).Format("20060102")
			}

			haveNumber, _ := regexp.MatchString("расписос на завтра (\\d[А-яA-z0-9]\\d)(\\-[А-яA-z0-9]{0,2})?", obj.Message.Text)

			if !haveNumber {
				bindFlag, groupNumber := getBinding(db, obj.Message.PeerID)
				if bindFlag {
					parseSchedule(groupNumber, date)
					message = formMessage(groupNumber, date)
				} else {
					message = "Использование: расписос на завтра *номер_группы*.\nДля получения подробной информации введите /help."
				}
			} else {
				re := regexp.MustCompile(`(\d[А-яA-z0-9]\d)(\-[А-яA-z0-9]{0,2})?`)
				groupNumber := re.FindString(obj.Message.Text)
				parseSchedule(groupNumber, date)
				message = formMessage(groupNumber, date)
			}

			// Собираем сообщение-ответ
			b := params.NewMessagesSendBuilder()
			b.Message(message)
			b.RandomID(0)
			b.PeerID(obj.Message.PeerID)
			vk.MessagesSend(b.Params)
			return
		}

		if strings.Contains(obj.Message.Text, "расписос") {

			// Если сообщение содержит текст "расписос", ожидается два варианта развития событий:
			// 1. Чат ассоциирован с группой
			// 2. Чат не ассоциирован с группой

			var date string

			// Проверка на выходной день
			if time.Now().Weekday().String() == "Sunday" {
				// Если сегодня воскресенье, то к дате прибавляется один день
				date = time.Now().AddDate(0, 0, 1).Format("20060102")
				message += "Сегодня воскресенье, но вот расписание на понедельник: \n"
			} else {
				// Иначе, используется сегодняшняя дата
				date = time.Now().Format("20060102")
			}

			// Необходимо найти номер группы в сообщении, с помощью регулярных выражений.
			//haveNumber, _ := regexp.MatchString("расписос (\\d(\\w|\\d)\\d)(\\-(\\w|\\d))?", obj.Message.Text)
			haveNumber, _ := regexp.MatchString("расписос (\\d[А-яA-z0-9]\\d)(\\-[А-яA-z0-9]{0,2})?", obj.Message.Text)

			if !haveNumber {

				// Если номер группы в сообщении, не обнаружен, необходимо проверить наличие ассоциации в БД.
				bindFlag, groupNumber := getBinding(db, obj.Message.PeerID)
				if bindFlag {

					// Если ассоциация найдена, необходимо передать переменную groupNumber
					// в parseSchedule() и formMessage(), для отправки расписания.
					parseSchedule(groupNumber, date)
					message = formMessage(groupNumber, date)
				} else {

					// Если номер группы не найден И ассоциации не существует, значит синтаксис команды неправильный,
					// а значит, формируется финальное сообщение с описанием синтаксиса.
					message = "Использование: расписос *номер_группы*.\nДля получения подробной информации введите /help."
				}
			} else {

				// Если номер группы найден в сообщении, необходимо передать переменную groupNumber
				// в parseSchedule() и formMessage(), для отправки расписания.
				re := regexp.MustCompile(`(\d[А-яA-z0-9]\d)(\-[А-яA-z0-9]{0,2})?`)
				groupNumber := re.FindString(obj.Message.Text)
				parseSchedule(groupNumber, date)
				message = formMessage(groupNumber, date)
			}

			// Собираем сообщение-ответ
			b := params.NewMessagesSendBuilder()
			b.Message(message)
			b.RandomID(0)
			b.PeerID(obj.Message.PeerID)
			vk.MessagesSend(b.Params)

		}
		message = ""
	})

	// Запуск lp-хендлера
	err = lp.Run()
	if err != nil {
		log.Fatal(err)
	}
}
