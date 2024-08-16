package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/joho/godotenv"
)

type Team struct {
	Username    string
	Password    string
	Stopwatch   time.Time
	StopwatchOn bool
}

type Quest struct {
	gorm.Model
	TeamName      string
	QuestNumber   int
	ImagePath     string
	Text          string
	CorrectAnswer string
	Completed     bool
}

var (
	db          *gorm.DB
	teams       = map[string]*Team{}
	mu          sync.Mutex
	templates   *template.Template
	templateDir = "../client"
)

func init() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	// Initialize SQLite database
	db, err = gorm.Open("sqlite3", "treasure_hunt.db")
	if err != nil {
		log.Fatalf("Failed to connect to the database: %v", err)
	}

	// Migrate the schema
	db.AutoMigrate(&Quest{})

	// Seed the database with quests
	seedDatabase()

	// Parse templates once and cache them
	templates = template.Must(template.ParseGlob(fmt.Sprintf("%s/*.html", templateDir)))
}

func main() {
	defer db.Close()

	// Initialize teams with credentials from environment variables
	teams["TEAM1"] = &Team{Username: os.Getenv("TEAM1USER"), Password: os.Getenv("TEAM1PASS")}
	teams["TEAM2"] = &Team{Username: os.Getenv("TEAM2USER"), Password: os.Getenv("TEAM2PASS")}
	teams["TEAM3"] = &Team{Username: os.Getenv("TEAM3USER"), Password: os.Getenv("TEAM3PASS")}

	// Serve static files
	http.Handle("/static/css/", http.StripPrefix("/static/css/", http.FileServer(http.Dir(fmt.Sprintf("%s/static/css", templateDir)))))
	http.Handle("/static/js/", http.StripPrefix("/static/js/", http.FileServer(http.Dir(fmt.Sprintf("%s/static/js", templateDir)))))
	http.Handle("/static/img/", http.StripPrefix("/static/img/", http.FileServer(http.Dir(fmt.Sprintf("%s/static/img", templateDir)))))
	// Serve the login page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			Message string
		}{
			Message: "Please enter your credentials",
		}

		err := templates.ExecuteTemplate(w, "index.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Handle the login form submission
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Parse the form data
			r.ParseForm()
			username := r.FormValue("username")
			password := r.FormValue("password")

			mu.Lock()
			defer mu.Unlock()

			// Authenticate the user and manage stopwatches
			for teamName, team := range teams {
				if username == team.Username && password == team.Password {
					if !team.StopwatchOn {
						team.Stopwatch = time.Now()
						team.StopwatchOn = true
					}

					// Set a session cookie to track the logged-in user
					http.SetCookie(w, &http.Cookie{
						Name:  "logged_in_team",
						Value: teamName,
						Path:  "/",
					})

					// Redirect to the treasure hunt page
					http.Redirect(w, r, fmt.Sprintf("/treasurehunt?team=%s", teamName), http.StatusSeeOther)
					return
				}
			}

			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		} else {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}
	})

	// Handle the treasure hunt page
	http.HandleFunc("/treasurehunt", func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie
		cookie, err := r.Cookie("logged_in_team")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teamName := cookie.Value
		requestedTeam := r.URL.Query().Get("team")
		success := r.URL.Query().Get("success")

		if teamName != requestedTeam {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		mu.Lock()
		team, ok := teams[teamName]
		mu.Unlock()

		if !ok {
			http.Error(w, "Invalid team", http.StatusBadRequest)
			return
		}

		elapsed := time.Since(team.Stopwatch)

		// Get the current quest
		var quest Quest
		if err := db.Where("team_name = ? AND completed = ?", teamName, false).Order("quest_number asc").First(&quest).Error; err != nil {
			// Redirect to the game finished page
			err = templates.ExecuteTemplate(w, "gamefinished.html", nil)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		var successMsg string
		var errorMsg string
		if success == "true" {
			successMsg = "Congratulations! You have successfully completed the quest."
		} else if success == "false" {
			errorMsg = "Wrong answer, try again!"
		}

		data := struct {
			Username    string
			StartTime   string
			ElapsedTime string
			Quest       Quest
			SuccessMsg  string
			ErrorMsg    string
		}{
			Username:    team.Username,
			StartTime:   team.Stopwatch.Format(time.RFC3339),
			ElapsedTime: elapsed.String(),
			Quest:       quest,
			SuccessMsg:  successMsg,
			ErrorMsg:    errorMsg,
		}

		err = templates.ExecuteTemplate(w, "treasurehunt.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			cookie, err := r.Cookie("logged_in_team")
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			teamName := cookie.Value

			// Parse form data
			r.ParseForm()
			answer := r.FormValue("answer")
			questID := r.FormValue("quest_id")

			// Retrieve the quest from the database using the quest_id and team_name
			var quest Quest
			if err := db.Where("id = ? AND team_name = ?", questID, teamName).First(&quest).Error; err != nil {
				log.Printf("Quest not found: %v", err)
				http.Error(w, "Quest not found", http.StatusNotFound)
				return
			}

			// Check the answer
			if answer == quest.CorrectAnswer {
				// Mark the current quest as completed
				quest.Completed = true
				db.Save(&quest)

				// Redirect to the next quest or show success message
				http.Redirect(w, r, fmt.Sprintf("/treasurehunt?team=%s&success=true", teamName), http.StatusSeeOther)
			} else {
				http.Redirect(w, r, fmt.Sprintf("/treasurehunt?team=%s&success=false", teamName), http.StatusSeeOther)
			}
		} else {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}
	})

	fmt.Println("Server is running on port 8080")
	http.ListenAndServe(":8080", nil)
}

func seedDatabase() {
	// Reset the Completed status of all quests
	db.Model(&Quest{}).Update("Completed", false)

	// Check if the quests are already in the database
	var count int64
	db.Model(&Quest{}).Count(&count)
	if count > 0 {
		fmt.Println("Database already seeded.")
		return
	}

	// Seed the database with quests and image paths
	quests := []Quest{
		{TeamName: "TEAM1", QuestNumber: 1, Text: "Find the hidden key.", CorrectAnswer: "key", ImagePath: "/static/img/key.png"},
		{TeamName: "TEAM1", QuestNumber: 2, Text: "Solve the ancient puzzle.", CorrectAnswer: "puzzle", ImagePath: "/static/img/puzzle.png"},
		{TeamName: "TEAM1", QuestNumber: 3, Text: "Navigate the maze to the treasure.", CorrectAnswer: "maze"},
		{TeamName: "TEAM2", QuestNumber: 1, Text: "Find the lost artifact.", CorrectAnswer: "artifact", ImagePath: "/static/images/artifact.png"},
		{TeamName: "TEAM2", QuestNumber: 2, Text: "Decode the ancient script.", CorrectAnswer: "decode", ImagePath: "/static/images/script.png"},
		{TeamName: "TEAM2", QuestNumber: 3, Text: "Escape the labyrinth.", CorrectAnswer: "escape", ImagePath: "/static/images/labyrinth.png"},
		{TeamName: "TEAM3", QuestNumber: 1, Text: "Discover the secret map.", CorrectAnswer: "map", ImagePath: "/static/images/map.png"},
		{TeamName: "TEAM3", QuestNumber: 2, Text: "Unlock the treasure chest.", CorrectAnswer: "chest", ImagePath: "/static/images/chest.png"},
		{TeamName: "TEAM3", QuestNumber: 3, Text: "Defeat the guardian.", CorrectAnswer: "guardian", ImagePath: "/static/images/guardian.png"},
	}

	for _, quest := range quests {
		db.Create(&quest)
	}

	fmt.Println("Database seeded with quests.")
}
