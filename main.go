package main

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
)

type Task struct {
	Title     string `json:"title"`
	ID        int    `json:"id"`
	Completed bool   `json:"completed"`
}

func main() {
	mux := http.NewServeMux()

	// Serve static files from the "static" directory
	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	mux.HandleFunc("/", index)
	mux.HandleFunc("/ios", handleIOS)
	mux.HandleFunc("/{name}", todo)
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})

	err := http.ListenAndServe(":3000", mux)
	if err != nil {
		log.Fatal(err)
	}
}

func todo(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	//strip "/" from path
	path = path[1:]
	if r.URL.Path == "favicon.ico" {
		return
	}

	log.Printf("Path: %s\n", path)
	completeTask(initDB(), path)
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func handleIOS(w http.ResponseWriter, r *http.Request) {
	db := initDB()
	defer db.Close()
	// Call getTasksJson to get the tasks as JSON
	tasksJson, err := getTasksJson(db)
	if err != nil {
		http.Error(w, "Error retrieving tasks", http.StatusInternalServerError)
		return
	}

	// Set the Content-Type to JSON and write the JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Write(tasksJson)
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	db := initDB()
	defer db.Close()

	switch r.Method {
	case http.MethodGet:
		tmpl, err := template.ParseFiles("templates/index.html")
		if err != nil {
			http.Error(w, "Error loading template", http.StatusInternalServerError)
			return
		}
		todos := getTasks(db)
		err = tmpl.Execute(w, todos)
		if err != nil {
			http.Error(w, "Error rendering template", http.StatusInternalServerError)
		}

	case http.MethodPost:
		log.Println("POST request")
		todo := r.FormValue("task")
		addTask(db, todo)
		http.Redirect(w, r, "/", http.StatusSeeOther)

	case http.MethodPut:
		log.Println("PUT request")
		todo := r.FormValue("task")
		completeTask(db, todo)

	case http.MethodOptions:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func initDB() *sql.DB {
	db, err := sql.Open("sqlite3", "./todo.db")
	if err != nil {
		log.Fatal(err)
	}

	// Create table if not exists
	query := `
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT,
		completed BOOLEAN
	);`
	_, err = db.Exec(query)
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func addTask(db *sql.DB, task string) {
	query := "INSERT INTO tasks (title, completed) VALUES (?, false)"
	_, err := db.Exec(query, task)
	if err != nil {
		log.Fatal(err)
	}
}

func getTasks(db *sql.DB) []Task {
	//I want to get title, id, and completed
	rows, err := db.Query("SELECT title, id, completed FROM tasks WHERE completed = false")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		task := Task{}
		err := rows.Scan(&task.Title, &task.ID, &task.Completed)
		if err != nil {
			log.Fatal(err)
		}
		tasks = append(tasks, task)
	}
	if len(tasks) == 0 {
		log.Println("No tasks found")
		return nil
	}

	return tasks
}

func getTasksJson(db *sql.DB) ([]byte, error) {
	// Query to get tasks from the database
	rows, err := db.Query("SELECT title, id, completed FROM tasks WHERE completed = false")
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	// Loop through the query results and populate the tasks slice
	for rows.Next() {
		task := Task{}
		err := rows.Scan(&task.Title, &task.ID, &task.Completed)
		if err != nil {
			log.Fatal(err)
			return nil, err
		}
		tasks = append(tasks, task)
	}
	// Check if tasks are empty
	if len(tasks) == 0 {
		log.Println("No tasks found")
		return nil, nil
	}

	// Marshal tasks into JSON
	tasksJson, err := json.Marshal(tasks)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return tasksJson, nil
}

func completeTask(db *sql.DB, id string) {
	query := "UPDATE tasks SET completed = true WHERE id = ?"
	_, err := db.Exec(query, id)
	if err != nil {
		log.Fatal(err)
	}
}

func getTaskByName(db *sql.DB, name string) (int, error) {
	var id int
	query := "SELECT id FROM tasks WHERE title = ?"
	err := db.QueryRow(query, name).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}
