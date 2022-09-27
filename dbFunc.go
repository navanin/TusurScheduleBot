package main

import (
	"database/sql"
	"fmt"
)

func getBinding(db *sql.DB, conversationId int) (bool, string) {

	// Функция для проверки наличия ассоциации группы с чатом ВК.
	// Происходит поиск в таблице по ID чата ВК, если запись имеется, то возвращается положительный результат и номер группы,
	// иначе, отрицательный рез-тат и пустая строка, вместо номера группы.

	var dbAnswer string
	db.QueryRow("select groupNumber from binds where groupid = ?", conversationId).Scan(&dbAnswer)

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
		return false
	} else {
		// Иначе, пара добавлена, функция возвращает положительный ответ.
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
		return false
	} else {
		// Иначе, пара удалена, функция возвращает положительный ответ.
		return true
	}
}

func getBindingsInfo(db *sql.DB) string {

	// Функция getBindingsInfo() отвечает за формирование сообщения со всеми ассоциациями.

	var message string
	var groupId string
	var groupNumber string
	var counter = 0

	// Выражение, для получения всех столбцов БД
	rows, _ := db.Query("select groupID, groupNumber from binds")

	message = "Актуальные ассоциации в БД:\n"

	defer rows.Close()
	for rows.Next() {

		counter++

		rows.Scan(&groupId, &groupNumber)
		message += fmt.Sprintf("%d. Чат %s - группа %s\n", counter, groupId, groupNumber)
	}
	return message
}
