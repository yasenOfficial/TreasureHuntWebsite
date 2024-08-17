package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/joho/godotenv"
)

type Team struct {
	Username     string
	Password     string
	Stopwatch    time.Time
	StopwatchOn  bool
	GameFinished bool
}

type Quest struct {
	gorm.Model
	TeamName      string
	QuestNumber   int
	ImagePath     string
	Text          string
	CorrectAnswer string
	Hint          string
	AudioPath     string
	Completed     bool
	Skipped       bool
	HintsUsed     int
	ImageRequired bool // New field to indicate if an image is required
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

	if _, err := os.Stat("uploads"); os.IsNotExist(err) {
		err := os.Mkdir("uploads", 0755)
		if err != nil {
			log.Fatalf("Failed to create uploads directory: %v", err)
		}
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
	http.Handle("/static/audio/", http.StripPrefix("/static/audio/", http.FileServer(http.Dir(fmt.Sprintf("%s/static/audio", templateDir)))))

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

	http.HandleFunc("/treasurehunt", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("logged_in_team")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		teamName := cookie.Value
		requestedTeam := r.URL.Query().Get("team")
		success := r.URL.Query().Get("success")
		skipped := r.URL.Query().Get("skipped")

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

		if team.GameFinished {

			var questCount int64
			db.Model(&Quest{}).Where("team_name = ? AND skipped = ?", teamName, true).Count(&questCount)

			var hintCount int64
			db.Model(&Quest{}).Where("team_name = ?", teamName).Select("sum(hints_used)").Row().Scan(&hintCount)

			http.Redirect(w, r, fmt.Sprintf("/gamefinished?team=%s&hintCount=%d&skipCount=%d", teamName, hintCount, questCount), http.StatusSeeOther)
			return
		}

		elapsed := time.Since(team.Stopwatch)

		// Get the total number of quests
		var totalQuests int64
		db.Model(&Quest{}).Where("team_name = ?", teamName).Count(&totalQuests)

		// Get the current quest
		var quest Quest
		if err := db.Where("team_name = ? AND completed = ?", teamName, false).Order("quest_number asc").First(&quest).Error; err != nil {
			// Redirect to the game finished page if no active quests
			var questCount int64
			db.Model(&Quest{}).Where("team_name = ? AND skipped = ?", teamName, true).Count(&questCount)

			var hintCount int64
			db.Model(&Quest{}).Where("team_name = ?", teamName).Select("sum(hints_used)").Row().Scan(&hintCount)

			http.Redirect(w, r, fmt.Sprintf("/gamefinished?team=%s&hintCount=%d&skipCount=%d", teamName, hintCount, questCount), http.StatusSeeOther)
			return
		}

		var successMsg string
		var errorMsg string
		var skipMsg string
		if success == "true" {
			successMsg = "Congratulations! You have successfully completed the quest."
		} else if success == "false" {
			errorMsg = "Wrong answer, try again!"
		} else if skipped == "true" {
			skipMsg = "You have skipped this quest."
		}

		data := struct {
			Username     string
			StartTime    string
			ElapsedTime  string
			Quest        Quest
			SuccessMsg   string
			ErrorMsg     string
			SkipMsg      string
			CurrentQuest int
			TotalQuests  int64
		}{
			Username:     team.Username,
			StartTime:    team.Stopwatch.Format(time.RFC3339),
			ElapsedTime:  elapsed.String(),
			Quest:        quest,
			SuccessMsg:   successMsg,
			ErrorMsg:     errorMsg,
			SkipMsg:      skipMsg,
			CurrentQuest: quest.QuestNumber,
			TotalQuests:  totalQuests,
		}

		err = templates.ExecuteTemplate(w, "treasurehunt.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Handle answer submission
	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			cookie, err := r.Cookie("logged_in_team")
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			teamName := cookie.Value

			// Parse form data
			err = r.ParseMultipartForm(10 << 20) // 10 MB limit for uploaded files
			if err != nil {
				http.Error(w, "Error parsing form data", http.StatusBadRequest)
				return
			}

			answer := r.FormValue("answer")
			questID := r.FormValue("quest_id")

			// Retrieve the quest from the database using the quest_id and team_name
			var quest Quest
			if err := db.Where("id = ? AND team_name = ?", questID, teamName).First(&quest).Error; err != nil {
				log.Printf("Quest not found: %v", err)
				http.Error(w, "Quest not found", http.StatusNotFound)
				return
			}

			if answer == "CODE=SKIP" {
				// Mark the quest as skipped
				quest.Skipped = true
				quest.Completed = true
				db.Save(&quest)
				logAction(quest.TeamName, fmt.Sprintf("Skipped Quest %d", quest.QuestNumber))
				http.Redirect(w, r, fmt.Sprintf("/treasurehunt?team=%s&skipped=true", teamName), http.StatusSeeOther)
				return
			}

			if quest.ImageRequired {
				file, handler, err := r.FormFile("uploaded_image")
				if err != nil {
					fmt.Println("No file uploaded")
					log.Printf("File upload error: %v", err)
					var totalQuests int64
					db.Model(&Quest{}).Where("team_name = ?", teamName).Count(&totalQuests)

					data := struct {
						Username     string
						StartTime    string
						ElapsedTime  string
						Quest        Quest
						SuccessMsg   string
						ErrorMsg     string
						SkipMsg      string
						CurrentQuest int
						TotalQuests  int64
					}{
						Username:     teams[teamName].Username,
						StartTime:    teams[teamName].Stopwatch.Format(time.RFC3339),
						ElapsedTime:  time.Since(teams[teamName].Stopwatch).String(),
						Quest:        quest,
						SuccessMsg:   "",
						ErrorMsg:     "No file uploaded",
						SkipMsg:      "",
						CurrentQuest: quest.QuestNumber,
						TotalQuests:  totalQuests,
					}
					templates.ExecuteTemplate(w, "treasurehunt.html", data)
					return
				}
				defer file.Close()

				// Save the file to the server
				filePath := fmt.Sprintf("uploads/%s_%d_%s", teamName, quest.QuestNumber, handler.Filename)
				dst, err := os.Create(filePath)
				if err != nil {
					fmt.Println("Error saving file, try again")
					log.Printf("Error saving file: %v", err)
					var totalQuests int64
					db.Model(&Quest{}).Where("team_name = ?", teamName).Count(&totalQuests)

					data := struct {
						Username     string
						StartTime    string
						ElapsedTime  string
						Quest        Quest
						SuccessMsg   string
						ErrorMsg     string
						SkipMsg      string
						CurrentQuest int
						TotalQuests  int64
					}{
						Username:     teams[teamName].Username,
						StartTime:    teams[teamName].Stopwatch.Format(time.RFC3339),
						ElapsedTime:  time.Since(teams[teamName].Stopwatch).String(),
						Quest:        quest,
						SuccessMsg:   "",
						ErrorMsg:     "Error saving file, try again",
						SkipMsg:      "",
						CurrentQuest: quest.QuestNumber,
						TotalQuests:  totalQuests,
					}
					templates.ExecuteTemplate(w, "treasurehunt.html", data)
					return
				}
				defer dst.Close()

				if _, err := io.Copy(dst, file); err != nil {
					fmt.Println("Error copying file")
					log.Printf("Error copying file: %v", err)

					var totalQuests int64
					db.Model(&Quest{}).Where("team_name = ?", teamName).Count(&totalQuests)

					data := struct {
						Username     string
						StartTime    string
						ElapsedTime  string
						Quest        Quest
						SuccessMsg   string
						ErrorMsg     string
						SkipMsg      string
						CurrentQuest int
						TotalQuests  int64
					}{
						Username:     teams[teamName].Username,
						StartTime:    teams[teamName].Stopwatch.Format(time.RFC3339),
						ElapsedTime:  time.Since(teams[teamName].Stopwatch).String(),
						Quest:        quest,
						SuccessMsg:   "",
						ErrorMsg:     "Error copying file",
						SkipMsg:      "",
						CurrentQuest: quest.QuestNumber,
						TotalQuests:  totalQuests,
					}
					templates.ExecuteTemplate(w, "treasurehunt.html", data)
					return
				}

				// Log the uploaded file path
				log.Printf("File uploaded successfully: %s", filePath)
			}

			// Check the answer
			if answer == quest.CorrectAnswer {
				// Mark the current quest as completed
				quest.Completed = true
				db.Save(&quest)
				logAction(quest.TeamName, fmt.Sprintf("Completed Quest %d", quest.QuestNumber))
				// Redirect to the next quest or show success message
				http.Redirect(w, r, fmt.Sprintf("/treasurehunt?team=%s&success=true", teamName), http.StatusSeeOther)
			} else {
				http.Redirect(w, r, fmt.Sprintf("/treasurehunt?team=%s&success=false", teamName), http.StatusSeeOther)
			}
		} else {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}
	})

	// Handler for hint requests
	http.HandleFunc("/hint/", func(w http.ResponseWriter, r *http.Request) {
		// Extract quest ID from the URL
		questID := r.URL.Path[len("/hint/"):]

		// Check for session cookie
		cookie, err := r.Cookie("logged_in_team")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		teamName := cookie.Value

		// Retrieve the quest from the database using the quest_id and team_name
		var quest Quest
		if err := db.Where("id = ? AND team_name = ?", questID, teamName).First(&quest).Error; err != nil {
			log.Printf("Quest not found: %v", err)
			http.Error(w, "Quest not found", http.StatusNotFound)
			return
		}

		// Increment the hint count and update the quest
		quest.HintsUsed++
		db.Save(&quest)

		// Log the hint usage
		logAction(teamName, fmt.Sprintf("Used Hint for Quest %d", quest.QuestNumber))

		// Respond with success
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success": true}`))
	})

	// Handle the game finished page
	http.HandleFunc("/gamefinished", func(w http.ResponseWriter, r *http.Request) {
		// Extract team name from query parameter
		teamName := r.URL.Query().Get("team")
		if teamName == "" {
			http.Error(w, "Team not specified", http.StatusBadRequest)
			return
		}

		// Retrieve quest stats from query parameters
		hintCount, err := strconv.ParseInt(r.URL.Query().Get("hintCount"), 10, 64)
		if err != nil {
			hintCount = 0
		}

		skipCount, err := strconv.ParseInt(r.URL.Query().Get("skipCount"), 10, 64)
		if err != nil {
			skipCount = 0
		}

		// Send the final results
		data := struct {
			HintCount int64
			SkipCount int64
		}{
			HintCount: hintCount,
			SkipCount: skipCount,
		}

		err = templates.ExecuteTemplate(w, "gamefinished.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	go func() {
		for {
			time.Sleep(5 * time.Second) // Check every 5 secs

			mu.Lock()
			for _, team := range teams {
				if team.StopwatchOn && time.Since(team.Stopwatch) >= 2*time.Hour { // FIX --------------------- THE TIME THE GAME WILL LAST --------------------------------------------
					team.GameFinished = true
					fmt.Printf("Team %s has finished the game\n", team.Username)
					// You can also log this or perform other actions
				}
			}
			mu.Unlock()
		}
	}()

	fmt.Println("Server is running on port 8080")
	http.ListenAndServe(":8080", nil)
}

// seedDatabase seeds the database with initial quests
func seedDatabase() {
	// // Reset the Completed status of all quests
	// db.Model(&Quest{}).Update("Completed", false)
	// db.Model(&Quest{}).Update("Skipped", false)
	// db.Model(&Quest{}).Update("HintsUsed", 0)
	// Clear the database
	db.Exec("DELETE FROM quests")

	// Check if the quests are already in the database
	var count int64
	db.Model(&Quest{}).Count(&count)
	if count > 0 {
		fmt.Println("Database already seeded.")
		return
	}

	// Seed the database with quests, including hints
	quests := []Quest{
		{TeamName: "TEAM1", QuestNumber: 1, Text: "Find the hidden key.", CorrectAnswer: "key", ImagePath: "/static/img/key.png", Hint: "Look where you least expect it."},
		{TeamName: "TEAM1", QuestNumber: 2, Text: "Solve the ancient puzzle.", CorrectAnswer: "puzzle", ImagePath: "/static/img/puzzle.png", Hint: "The answer lies in the patterns.", ImageRequired: true},
		{TeamName: "TEAM1", QuestNumber: 3, Text: "Navigate the maze to the treasure.", CorrectAnswer: "maze", AudioPath: "/static/audio/maze.mp3"},
		{TeamName: "TEAM1", QuestNumber: 4, Text: "Discover the sea.", CorrectAnswer: "sea", Hint: "The answer is in the image.", ImagePath: "/static/img/sea.jpg"},
		{TeamName: "TEAM2", QuestNumber: 1, Text: "Find the lost artifact.", CorrectAnswer: "artifact", ImagePath: "/static/images/artifact.png", Hint: "Think about ancient history."},
		{TeamName: "TEAM2", QuestNumber: 2, Text: "Decode the ancient script.", CorrectAnswer: "decode", ImagePath: "/static/images/script.png"},
		{TeamName: "TEAM2", QuestNumber: 3, Text: "Escape the labyrinth.", CorrectAnswer: "escape", ImagePath: "/static/images/labyrinth.png", Hint: "Follow the left wall."},
		{TeamName: "TEAM3", QuestNumber: 1, Text: "Discover the secret map.", CorrectAnswer: "map", ImagePath: "/static/images/map.png"},
		{TeamName: "TEAM3", QuestNumber: 2, Text: "Unlock the treasure chest.", CorrectAnswer: "chest", ImagePath: "/static/images/chest.png", Hint: "The key is hidden nearby."},
		{TeamName: "TEAM3", QuestNumber: 3, Text: "Defeat the guardian.", CorrectAnswer: "guardian", ImagePath: "/static/images/guardian.png"},
	}

	for _, quest := range quests {
		db.Create(&quest)
	}

	fmt.Println("Database seeded with quests.")
}

// logAction logs team actions to a file
func logAction(teamName, action string) {
	file, err := os.OpenFile("team_actions.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Error opening log file: %v", err)
		return
	}
	defer file.Close()

	log.SetOutput(file)
	log.Printf("Team %s: %s\n", teamName, action)
}
