package djrouter

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/tomicleveling/core/pkg/authenticator"
	"github.com/tomicleveling/core/pkg/database"
)

func InitRouter(auth *authenticator.Authenticator) *http.ServeMux {
	router := http.NewServeMux()

	// Serve static files from the "static" directory
	fileServer := http.FileServer(http.Dir("static"))
	router.Handle("/static/", http.StripPrefix("/static/", fileServer))

	router.HandleFunc("/profile", serveProfile)
	router.HandleFunc("/login", loginHandler(auth))
	router.HandleFunc("/callback", callbackHandler(auth))
	router.HandleFunc("/logout", logoutHandler(auth))
	router.HandleFunc("/quick", serveQuicktasks)
	router.HandleFunc("/score", score)
	router.HandleFunc("/db", updateDB)
	router.HandleFunc("/", index)
	router.HandleFunc("/hook", handleHook)
	router.HandleFunc("/ios", handleIOS)
	router.HandleFunc("/{name}", todo)
	router.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})

	return router
}

func handleHook(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("nohup", "/bin/bash", "./cicd.sh", "&")
	log.Println(cmd)
}

func updateDB(w http.ResponseWriter, r *http.Request) {
	database.AlterDB()
}

func score(w http.ResponseWriter, r *http.Request) {
	user, err := getProfileCookie(r)
	if err != nil {
		log.Println(err)
	}

	log.Println(user)
	w.Header().Set("Content-Type", "text/html") // Ensure response is HTML
	xp := database.GetScore(database.InitDB(), user)
	level := GetLevel(xp)
	score := fmt.Sprintf("<h3>LEVEL: %d</h3><h3>SCORE: %d</h3>", level, xp)
	fmt.Fprintf(w, score)
}

func logoutHandler(auth *authenticator.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logoutUrl, err := url.Parse("https://" + os.Getenv("AUTH0_DOMAIN") + "/v2/logout")
		if err != nil {
			log.Println(err)
		}
		// Set the redirect URI after logout (home page in this case)
		redirectUri := os.Getenv("AUTH0_REDIRECT_URL") // Change this to your home page URL
		queryParams := url.Values{}
		queryParams.Add("returnTo", redirectUri)    // 'returnTo' parameter for Auth0 to redirect after logout
		queryParams.Add("client_id", auth.ClientID) // Add client_id (required by Auth0)

		// Add query parameters to logout URL
		logoutUrl.RawQuery = queryParams.Encode()
		// delete cookie
		http.SetCookie(w, &http.Cookie{
			Name:  "access_token",
			Value: "",
		})
		//delete state
		http.SetCookie(w, &http.Cookie{
			Name:  "state",
			Value: "",
		})
		http.SetCookie(w, &http.Cookie{
			Name:  "profile",
			Value: "",
		})

		http.Redirect(w, r, logoutUrl.String(), http.StatusTemporaryRedirect)
	}
}

func callbackHandler(auth *authenticator.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Exchange an authorization code for a token.
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		// Extract the state and code from the request URL (query parameters)
		queryParams := r.URL.Query()
		code := queryParams.Get("code")
		state := queryParams.Get("state")
		// Get the state cookie if available (for CSRF protection)
		cookie, err := r.Cookie("state")
		if err != nil {
			http.Error(w, "State cookie missing", http.StatusBadRequest)
			return
		}

		// Check if the state from the query matches the state cookie
		if cookie.Value != state {
			http.Error(w, "State mismatch", http.StatusForbidden)
			return
		}

		// Exchange an authorization code for a token.
		token, err := auth.Exchange(ctx, code)
		if err != nil {
			log.Println(err)
		}

		idToken, err := auth.VerifyIDToken(ctx, token)
		if err != nil {
			log.Println(err)
		}

		cookie = &http.Cookie{
			Name:  "access_token",
			Value: token.AccessToken,
		}
		http.SetCookie(w, cookie)

		var profile map[string]interface{}
		if err := idToken.Claims(&profile); err != nil {
			log.Println(err)
		}

		// Serialize the profile to JSON
		profileJSON, err := json.Marshal(profile)
		if err != nil {
			log.Println("Error serializing profile:", err)
			return
		}

		encoded := url.QueryEscape(string(profileJSON))
		// Create the cookie
		cookie2 := &http.Cookie{
			Name:  "profile",
			Value: encoded, // Store the serialized JSON string
			Path:  "/",
		}

		log.Println(cookie2.Value)
		log.Println("COOKIE6")
		// Set the cookie in the response
		http.SetCookie(w, cookie2)
		log.Println(cookie2.Value)

		log.Println("COOKIE7")
		_, err = auth.VerifyIDToken(r.Context(), token)
		if err != nil {
			log.Println(err)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}

}

func loginHandler(auth *authenticator.Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, err := generateRandomState()
		if err != nil {
			log.Println(err)
		}

		//create a cookie and store state
		cookie := &http.Cookie{
			Name:  "state",
			Value: state,
		}
		http.SetCookie(w, cookie)

		// Use custom Auth0 API to enable RBAC/Permissions
		http.Redirect(w, r, auth.AuthCodeURL(state, oauth2.SetAuthURLParam("audience", os.Getenv("AUTH0_AUDIENCE"))), http.StatusSeeOther)

	}
}

func serveProfile(w http.ResponseWriter, r *http.Request) {
	isAuthed := isAuthenticated(r)
	var data []string
	if !isAuthed {
		data = []string{"UNKNOWN USER PLEASE LOG IN"}
	} else {
		profile, err := getProfileCookie(r)
		if err != nil {
			log.Printf("Error getting profile cookie: %v\n", err)
		} else {
			log.Printf("Profile: %v\n", profile)
			data = []string{profile}
		}
	}

	tmpl, err := template.ParseFiles("templates/user.html")
	if err != nil {
		http.Error(w, "Error loading template", http.StatusInternalServerError)
		return
	}
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	state := base64.StdEncoding.EncodeToString(b)

	return state, nil
}

func isAuthenticated(r *http.Request) bool {
	// Check if the user's access token cookie is present
	cookie, err := r.Cookie("access_token")
	if err != nil {
		return false // Not authenticated if the cookie is not found
	}
	if cookie.Value == "" {
		return false // Not authenticated if the cookie value is empty
	}

	// TODO: Should validate access token
	extractPermissions(cookie.Value)
	return true
}

func getProfileCookie(r *http.Request) (string, error) {
	// Now let's retrieve the cookie
	retrievedCookie, err := r.Cookie("profile")
	if err != nil {
		log.Println("Error retrieving cookie:", err)
	}

	if retrievedCookie.Value == "" {
		log.Println("Cookie not found")
		return "", fmt.Errorf("cookie not found")
	}

	decoded, err := url.QueryUnescape(retrievedCookie.Value)
	if err != nil {
		log.Println(err)
	}
	var profile map[string]string
	if err := json.Unmarshal([]byte(decoded), &profile); err != nil {
		log.Println("It works?")
	}

	// Return nickname
	return profile["nickname"], nil
}

// Function to parse and extract permissions
func extractPermissions(accessToken string) {
	token, _, err := jwt.NewParser().ParseUnverified(accessToken, jwt.MapClaims{})
	if err != nil {
		log.Fatalf("Error parsing token: %v", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if permissions, exists := claims["permissions"]; exists {
			fmt.Println("Permissions:", permissions)
		} else {
			fmt.Println("No permissions claim found in token")
		}
	} else {
		fmt.Println("Invalid token claims")
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
	user, err := getProfileCookie(r)
	if err != nil {
		log.Println(err)
	}
	database.CompleteTask(database.InitDB(), path, user)
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

func handleIOS(w http.ResponseWriter, r *http.Request) {
	db := database.InitDB()
	defer db.Close()
	// Call getTasksJson to get the tasks as JSON
	user, err := getProfileCookie(r)
	if err != nil {
		log.Println(err)
	}
	tasksJson, err := database.GetTasksJson(db, user)
	if err != nil {
		http.Error(w, "Error retrieving tasks", http.StatusInternalServerError)
		return
	}

	// Set the Content-Type to JSON and write the JSON response
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(tasksJson)
	if err != nil {
		log.Println(err)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	db := database.InitDB()
	defer db.Close()

	switch r.Method {
	case http.MethodGet:
		user, err := getProfileCookie(r)
		if err != nil {
			log.Println(err)
		}
		tmpl, err := template.ParseFiles("templates/index.html", "templates/empty.html")
		if err != nil {
			http.Error(w, "Error loading template", http.StatusInternalServerError)
			return
		}
		todos := database.GetTasks(db, user)
		err = tmpl.Execute(w, todos)
		if err != nil {
			http.Error(w, "Error rendering template", http.StatusInternalServerError)
		}

	case http.MethodPost:
		log.Println("POST request")
		user, err := getProfileCookie(r)
		if err != nil {
			log.Println(err)
		}
		todo := r.FormValue("task")
		database.AddTask(db, todo, user)
		http.Redirect(w, r, "/quick", http.StatusSeeOther)

	case http.MethodPut:
		log.Println("PUT request")
		user, err := getProfileCookie(r)
		if err != nil {
			log.Println(err)
		}
		todo := r.FormValue("task")
		database.CompleteTask(db, todo, user)

	case http.MethodOptions:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
func serveQuicktasks(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/quick" {
		http.NotFound(w, r)
		return
	}
	db := database.InitDB()
	defer db.Close()

	switch r.Method {
	case http.MethodGet:
		user, err := getProfileCookie(r)
		if err != nil {
			log.Println(err)
		}
		tmpl, err := template.ParseFiles("templates/index.html", "templates/quicktasks.html")
		if err != nil {
			http.Error(w, "Error loading template", http.StatusInternalServerError)
			return
		}
		todos := database.GetTasks(db, user)
		err = tmpl.Execute(w, todos)
		if err != nil {
			http.Error(w, "Error rendering template", http.StatusInternalServerError)
		}

	case http.MethodPost:
		log.Println("POST request")
		user, err := getProfileCookie(r)
		if err != nil {
			log.Println(err)
		}
		todo := r.FormValue("task")
		database.AddTask(db, todo, user)
		http.Redirect(w, r, "/quick", http.StatusSeeOther)

	case http.MethodPut:
		log.Println("PUT request")
		user, err := getProfileCookie(r)
		if err != nil {
			log.Println(err)
		}
		todo := r.FormValue("task")
		database.CompleteTask(db, todo, user)

	case http.MethodOptions:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func GetLevel(xp int) int {
	baseXP := 10.0 // XP required for level 1
	level := 1

	for xp >= int(baseXP) {
		level++
		baseXP *= 1.5 // Increase XP requirement by 50% each level
	}

	return level - 1 // Adjust because loop increments once past the max level
}
