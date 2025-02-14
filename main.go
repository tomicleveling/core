package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
)

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
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleIOS(w http.ResponseWriter, r *http.Request) {
	db := initDB()
	defer db.Close()
	todos := getTasks(db)
	tmpl, err := template.ParseFiles("templates/ios.html")
	if err != nil {
		http.Error(w, "Error loading template", http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, todos)
	if err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
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

func getTasks(db *sql.DB) []string {
	rows, err := db.Query("SELECT title FROM tasks WHERE completed = false")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var tasks []string
	for rows.Next() {
		var title string
		err := rows.Scan(&title)
		if err != nil {
			log.Fatal(err)
		}
		tasks = append(tasks, title)
	}
	if len(tasks) == 0 {
		log.Println("No tasks found")
		return []string{}
	}

	return tasks
}

func completeTask(db *sql.DB, name string) {
	id, err := getTaskByName(db, name)
	if err != nil {
		log.Fatal(err)
	}
	query := "UPDATE tasks SET completed = true WHERE id = ?"
	_, err = db.Exec(query, id)
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
