package database

import (
	"database/sql"
	"encoding/json"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type Task struct {
	Title     string `json:"title"`
	ID        int    `json:"id"`
	Completed bool   `json:"completed"`
	User      string `json:"user"`
}

func InitDB() *sql.DB {
	db, err := sql.Open("sqlite3", "./todo.db")
	if err != nil {
		log.Fatal(err)
	}

	// Create table if not exists
	query := `
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT,
		completed BOOLEAN,
		user TEXT
	);`
	_, err = db.Exec(query)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func AddTask(db *sql.DB, task, user string) {
	query := "INSERT INTO tasks (title, completed, user) VALUES (?, ?, ?)"
	_, err := db.Exec(query, task, 0, user)
	if err != nil {
		log.Fatal(err)
	}
}

func GetTasks(db *sql.DB, user string) []Task {
	//I want to get title, id, and completed
	rows, err := db.Query("SELECT title, id, completed, user FROM tasks WHERE completed = 0 AND user = ?", user)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		task := Task{}
		err := rows.Scan(&task.Title, &task.ID, &task.Completed, &task.User)
		if err != nil {
			log.Fatal(err)
		}
		tasks = append(tasks, task)
	}
	if len(tasks) == 0 {
		log.Println("No tasks found for user: ", user)
		return nil
	}

	return tasks
}

func ParseTask(tasks []Task) ( []byte, error) {

	tasksJson, err := json.Marshal(tasks)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return tasksJson, nil
}


func CompleteTask(db *sql.DB, id, user string) {
	query := "UPDATE tasks SET completed = true WHERE id = ? AND user = ?"
	_, err := db.Exec(query, id, user)
	if err != nil {
		log.Fatal(err)
	}
}

func GetTaskByName(db *sql.DB, name string) (int, error) {
	var id int
	query := "SELECT id FROM tasks WHERE title = ?"
	err := db.QueryRow(query, name).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func EditTask(db *sql.DB, user,title, task string) error{
	query := "UPDATE tasks SET title=? WHERE title = ? AND user = ?"
	_,err := db.Exec(query, title, task, user)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}
