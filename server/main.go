package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	TeamName       string
	QuestNumber    int
	ImagePath      string
	Text           string
	CorrectAnswers string
	Hint           string
	AudioPath      string
	Completed      bool
	Skipped        bool
	HintsUsed      int
	FileRequired   bool

	QuestTimerRequired bool
	QuestTimerDuration time.Duration
	QuestTimerEndTime  time.Time
	QuestTimerRunning  bool
	QuestTimerFinished bool

	HintTimerRequired bool
	HintTimerDuration time.Duration
	HintTimerEndTime  time.Time
	HintTimerRunning  bool
	HintTimerFinished bool
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
			// Message: "Please enter your credentials",
			Message: "Моля, въведете вашите данни за вход",
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

			// http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			http.Error(w, "Невалидни данни за вход", http.StatusUnauthorized)
		} else {
			// http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			http.Error(w, "Невалидни данни за вход", http.StatusUnauthorized)
		}
	})

	http.HandleFunc("/treasurehunt", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("logged_in_team")
		if err != nil {
			// http.Error(w, "Unauthorized", http.StatusUnauthorized)
			http.Error(w, "Неоторизиран достъп", http.StatusUnauthorized)

			return
		}

		teamName := cookie.Value
		requestedTeam := r.URL.Query().Get("team")
		success := r.URL.Query().Get("success")
		skipped := r.URL.Query().Get("skipped")

		if teamName != requestedTeam {
			// http.Error(w, "Unauthorized", http.StatusUnauthorized)
			http.Error(w, "Неоторизиран достъп", http.StatusUnauthorized)

			return
		}

		mu.Lock()
		team, ok := teams[teamName]
		mu.Unlock()

		if !ok {
			// http.Error(w, "Invalid team", http.StatusBadRequest)
			http.Error(w, "Невалиден отбор", http.StatusBadRequest)

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
		if quest.HintTimerRequired && !quest.HintTimerRunning && !quest.HintTimerFinished {
			quest.HintTimerEndTime = time.Now().Add(quest.HintTimerDuration)
			quest.HintTimerRunning = true
			quest.HintTimerFinished = false
			db.Save(&quest)
			// fmt.Println("Timer started for hint", quest.QuestNumber)
			// fmt.Println("Timer will end at", quest.HintTimerEndTime)
			// fmt.Println("Timer duration", quest.HintTimerDuration)
			// fmt.Println("Time remaining", time.Until(quest.HintTimerEndTime))
		}

		var hintTimerRemaining string
		if quest.HintTimerRequired {
			remaining := time.Until(quest.HintTimerEndTime)
			if remaining > 0 {
				hintTimerRemaining = remaining.String()
			} else {
				quest.HintTimerRunning = false
				quest.HintTimerFinished = true
				db.Save(&quest)
			}
		}

		if quest.QuestTimerRequired && !quest.QuestTimerRunning && !quest.QuestTimerFinished {
			quest.QuestTimerEndTime = time.Now().Add(quest.QuestTimerDuration)
			quest.QuestTimerRunning = true
			quest.QuestTimerFinished = false
			db.Save(&quest)
			// fmt.Println("Timer started for quest", quest.QuestNumber)
			// fmt.Println("Timer will end at", quest.QuestTimerEndTime)
			// fmt.Println("Timer duration", quest.QuestTimerDuration)
			// fmt.Println("Time remaining", time.Until(quest.QuestTimerEndTime))
		}

		var questTimerRemaining string
		if quest.QuestTimerRequired {
			remaining := time.Until(quest.QuestTimerEndTime)
			if remaining > 0 {
				questTimerRemaining = remaining.String()
			} else {
				quest.QuestTimerRunning = false
				quest.QuestTimerFinished = true
				db.Save(&quest)

				data := struct {
					Username            string
					StartTime           string
					ElapsedTime         string
					Quest               Quest
					SuccessMsg          string
					ErrorMsg            string
					SkipMsg             string
					CurrentQuest        int
					TotalQuests         int64
					QuestTimerRemaining string
					QuestTimerEndTime   string
					HintTimerRemaining  string
					HintTimerEndTime    string
				}{
					Username:            team.Username,
					StartTime:           team.Stopwatch.Format(time.RFC3339),
					ElapsedTime:         elapsed.String(),
					Quest:               quest,
					SuccessMsg:          "Quest timer has ended!",
					ErrorMsg:            "",
					SkipMsg:             "",
					CurrentQuest:        quest.QuestNumber,
					TotalQuests:         totalQuests,
					QuestTimerRemaining: questTimerRemaining,
					QuestTimerEndTime:   quest.QuestTimerEndTime.Format(time.RFC3339),
					HintTimerRemaining:  hintTimerRemaining,
					HintTimerEndTime:    quest.HintTimerEndTime.Format(time.RFC3339),
				}

				err = templates.ExecuteTemplate(w, "treasurehunt.html", data)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}

				return
			}
		}

		var successMsg string
		var errorMsg string
		var skipMsg string
		if success == "true" {
			// successMsg = "Congratulations! You have successfully completed the quest."
			successMsg = "Поздравления! Успешно завършихте задачата."
		} else if success == "false" {
			// errorMsg = "Wrong answer, try again!"
			errorMsg = "Грешен отговор, опитайте отново!"
		} else if skipped == "true" {
			// skipMsg = "You have skipped this quest."
			skipMsg = "Прескочихте тази задача."
		}

		data := struct {
			Username            string
			StartTime           string
			ElapsedTime         string
			Quest               Quest
			SuccessMsg          string
			ErrorMsg            string
			SkipMsg             string
			CurrentQuest        int
			TotalQuests         int64
			QuestTimerRemaining string
			QuestTimerEndTime   string
			HintTimerRemaining  string
			HintTimerEndTime    string
		}{
			Username:            team.Username,
			StartTime:           team.Stopwatch.Format(time.RFC3339),
			ElapsedTime:         elapsed.String(),
			Quest:               quest,
			SuccessMsg:          successMsg,
			ErrorMsg:            errorMsg,
			SkipMsg:             skipMsg,
			CurrentQuest:        quest.QuestNumber,
			TotalQuests:         totalQuests,
			QuestTimerRemaining: questTimerRemaining,
			QuestTimerEndTime:   quest.QuestTimerEndTime.Format(time.RFC3339),
			HintTimerRemaining:  hintTimerRemaining,
			HintTimerEndTime:    quest.HintTimerEndTime.Format(time.RFC3339),
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
				// http.Error(w, "Unauthorized", http.StatusUnauthorized)
				http.Error(w, "Неавторизиран достъп", http.StatusUnauthorized)

				return
			}
			teamName := cookie.Value

			// Parse form data
			err = r.ParseMultipartForm(10 << 20) // 10 MB limit for uploaded files
			if err != nil {
				// http.Error(w, "Error parsing form data", http.StatusBadRequest)
				http.Error(w, "Грешка при обработката на формуляра", http.StatusBadRequest)

				return
			}

			answer := r.FormValue("answer")
			questID := r.FormValue("quest_id")

			// Retrieve the quest from the database using the quest_id and team_name
			var quest Quest
			if err := db.Where("id = ? AND team_name = ?", questID, teamName).First(&quest).Error; err != nil {
				log.Printf("Quest not found: %v", err)
				// http.Error(w, "Quest not found", http.StatusNotFound)
				http.Error(w, "Задачата не е намерена", http.StatusNotFound)
				return
			}

			if quest.QuestTimerRequired && time.Now().Before(quest.QuestTimerEndTime) {
				var totalQuests int64
				db.Model(&Quest{}).Where("team_name = ?", teamName).Count(&totalQuests)

				data := struct {
					Username            string
					StartTime           string
					ElapsedTime         string
					Quest               Quest
					SuccessMsg          string
					ErrorMsg            string
					SkipMsg             string
					CurrentQuest        int
					TotalQuests         int64
					QuestTimerRemaining string
					QuestTimerEndTime   string
					HintTimerRemaining  string
					HintTimerEndTime    string
				}{
					Username:    teams[teamName].Username,
					StartTime:   teams[teamName].Stopwatch.Format(time.RFC3339),
					ElapsedTime: time.Since(teams[teamName].Stopwatch).String(),
					Quest:       quest,
					SuccessMsg:  "",
					// ErrorMsg:     "Wait for the quest timer to end!",
					ErrorMsg:     "Изчакайте таймера на задачата да изтече!",
					SkipMsg:      "",
					CurrentQuest: quest.QuestNumber,
					TotalQuests:  totalQuests,
					QuestTimerRemaining: func() string {
						if quest.QuestTimerRunning {
							return time.Until(quest.QuestTimerEndTime).String()
						}
						return ""
					}(),
					QuestTimerEndTime: func() string {
						if quest.QuestTimerRunning {
							return quest.QuestTimerEndTime.Format(time.RFC3339)
						}
						return ""
					}(),
					HintTimerRemaining: func() string {
						if quest.HintTimerRunning {
							return time.Until(quest.HintTimerEndTime).String()
						}
						return ""
					}(),
					HintTimerEndTime: func() string {
						if quest.HintTimerRunning {
							return quest.HintTimerEndTime.Format(time.RFC3339)
						}
						return ""
					}(),
				}
				templates.ExecuteTemplate(w, "treasurehunt.html", data)
				return
			}

			correctAnswers := strings.Split(quest.CorrectAnswers, "|")
			isCorrect := false
			for _, correctAnswer := range correctAnswers {
				if strings.TrimSpace(strings.ToLower(answer)) == strings.TrimSpace(strings.ToLower(correctAnswer)) {
					isCorrect = true
					break
				}
			}

			if strings.ToLower(answer) == "skip" {
				// Mark the quest as skipped
				quest.Skipped = true
				quest.Completed = true
				db.Save(&quest)
				logAction(quest.TeamName, fmt.Sprintf("Skipped Quest %d", quest.QuestNumber))
				http.Redirect(w, r, fmt.Sprintf("/treasurehunt?team=%s&skipped=true", teamName), http.StatusSeeOther)
				return
			}

			if quest.FileRequired {
				file, handler, err := r.FormFile("uploaded_image")
				if err != nil {
					// fmt.Println("No file uploaded")
					log.Printf("File upload error: %v", err)
					var totalQuests int64
					db.Model(&Quest{}).Where("team_name = ?", teamName).Count(&totalQuests)

					data := struct {
						Username            string
						StartTime           string
						ElapsedTime         string
						Quest               Quest
						SuccessMsg          string
						ErrorMsg            string
						SkipMsg             string
						CurrentQuest        int
						TotalQuests         int64
						QuestTimerRemaining string
						QuestTimerEndTime   string
						HintTimerRemaining  string
						HintTimerEndTime    string
					}{
						Username:    teams[teamName].Username,
						StartTime:   teams[teamName].Stopwatch.Format(time.RFC3339),
						ElapsedTime: time.Since(teams[teamName].Stopwatch).String(),
						Quest:       quest,
						SuccessMsg:  "",
						// ErrorMsg:     "No file uploaded",
						ErrorMsg:     "Не е качен файл",
						SkipMsg:      "",
						CurrentQuest: quest.QuestNumber,
						TotalQuests:  totalQuests,
						QuestTimerRemaining: func() string {
							if quest.QuestTimerRunning {
								return time.Until(quest.QuestTimerEndTime).String()
							}
							return ""
						}(),
						QuestTimerEndTime: func() string {
							if quest.QuestTimerRunning {
								return quest.QuestTimerEndTime.Format(time.RFC3339)
							}
							return ""
						}(),
						HintTimerRemaining: func() string {
							if quest.HintTimerRunning {
								return time.Until(quest.HintTimerEndTime).String()
							}
							return ""
						}(),
						HintTimerEndTime: func() string {
							if quest.HintTimerRunning {
								return quest.HintTimerEndTime.Format(time.RFC3339)
							}
							return ""
						}(),
					}
					templates.ExecuteTemplate(w, "treasurehunt.html", data)
					return
				}
				defer file.Close()

				// Save the file to the server
				filePath := fmt.Sprintf("uploads/%s_%d_%s", teamName, quest.QuestNumber, handler.Filename)
				dst, err := os.Create(filePath)
				if err != nil {
					// fmt.Println("Error saving file, try again")
					log.Printf("Error saving file: %v", err)
					var totalQuests int64
					db.Model(&Quest{}).Where("team_name = ?", teamName).Count(&totalQuests)

					data := struct {
						Username            string
						StartTime           string
						ElapsedTime         string
						Quest               Quest
						SuccessMsg          string
						ErrorMsg            string
						SkipMsg             string
						CurrentQuest        int
						TotalQuests         int64
						QuestTimerRemaining string
						QuestTimerEndTime   string
						HintTimerRemaining  string
						HintTimerEndTime    string
					}{
						Username:    teams[teamName].Username,
						StartTime:   teams[teamName].Stopwatch.Format(time.RFC3339),
						ElapsedTime: time.Since(teams[teamName].Stopwatch).String(),
						Quest:       quest,
						SuccessMsg:  "",
						// ErrorMsg:     "Error saving file, try again",
						ErrorMsg:     "Грешка при запазването на файла, опитайте отново",
						SkipMsg:      "",
						CurrentQuest: quest.QuestNumber,
						TotalQuests:  totalQuests,
						QuestTimerRemaining: func() string {
							if quest.QuestTimerRunning {
								return time.Until(quest.QuestTimerEndTime).String()
							}
							return ""
						}(),
						QuestTimerEndTime: func() string {
							if quest.QuestTimerRunning {
								return quest.QuestTimerEndTime.Format(time.RFC3339)
							}
							return ""
						}(),
						HintTimerRemaining: func() string {
							if quest.HintTimerRunning {
								return time.Until(quest.HintTimerEndTime).String()
							}
							return ""
						}(),
						HintTimerEndTime: func() string {
							if quest.HintTimerRunning {
								return quest.HintTimerEndTime.Format(time.RFC3339)
							}
							return ""
						}(),
					}
					templates.ExecuteTemplate(w, "treasurehunt.html", data)
					return
				}
				defer dst.Close()

				if _, err := io.Copy(dst, file); err != nil {
					// fmt.Println("Error copying file")
					log.Printf("Error copying file: %v", err)

					var totalQuests int64
					db.Model(&Quest{}).Where("team_name = ?", teamName).Count(&totalQuests)

					data := struct {
						Username            string
						StartTime           string
						ElapsedTime         string
						Quest               Quest
						SuccessMsg          string
						ErrorMsg            string
						SkipMsg             string
						CurrentQuest        int
						TotalQuests         int64
						QuestTimerRemaining string
						QuestTimerEndTime   string
						HintTimerRemaining  string
						HintTimerEndTime    string
					}{
						Username:    teams[teamName].Username,
						StartTime:   teams[teamName].Stopwatch.Format(time.RFC3339),
						ElapsedTime: time.Since(teams[teamName].Stopwatch).String(),
						Quest:       quest,
						SuccessMsg:  "",
						// ErrorMsg:     "Error copying file",
						ErrorMsg:     "Грешка при копирането на файла",
						SkipMsg:      "",
						CurrentQuest: quest.QuestNumber,
						TotalQuests:  totalQuests,
						QuestTimerRemaining: func() string {
							if quest.QuestTimerRunning {
								return time.Until(quest.QuestTimerEndTime).String()
							}
							return ""
						}(),
						QuestTimerEndTime: func() string {
							if quest.QuestTimerRunning {
								return quest.QuestTimerEndTime.Format(time.RFC3339)
							}
							return ""
						}(),
						HintTimerRemaining: func() string {
							if quest.HintTimerRunning {
								return time.Until(quest.HintTimerEndTime).String()
							}
							return ""
						}(),
						HintTimerEndTime: func() string {
							if quest.HintTimerRunning {
								return quest.HintTimerEndTime.Format(time.RFC3339)
							}
							return ""
						}(),
					}
					templates.ExecuteTemplate(w, "treasurehunt.html", data)
					return
				}

				// Log the uploaded file path
				log.Printf("File uploaded successfully: %s", filePath)
			}

			// Check the answer
			if isCorrect {
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
			// http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			http.Error(w, "Невалиден метод на заявка", http.StatusMethodNotAllowed)
		}
	})

	// Handler for hint requests
	http.HandleFunc("/hint/", func(w http.ResponseWriter, r *http.Request) {
		// Extract quest ID from the URL
		questID := r.URL.Path[len("/hint/"):]

		// Check for session cookie
		cookie, err := r.Cookie("logged_in_team")
		if err != nil {
			// http.Error(w, "Unauthorized", http.StatusUnauthorized)
			http.Error(w, "Неоторизиран достъп", http.StatusUnauthorized)
			return
		}
		teamName := cookie.Value

		// Retrieve the quest from the database using the quest_id and team_name
		var quest Quest
		if err := db.Where("id = ? AND team_name = ?", questID, teamName).First(&quest).Error; err != nil {
			log.Printf("Quest not found: %v", err)
			// http.Error(w, "Quest not found", http.StatusNotFound)
			http.Error(w, "Задачата не е намерена", http.StatusNotFound)
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
			// http.Error(w, "Team not specified", http.StatusBadRequest)
			http.Error(w, "Отборът не е посочен", http.StatusBadRequest)
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

	http.HandleFunc("/check-quest-status", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("logged_in_team")
		if err != nil {
			http.Error(w, "Неоторизиран достъп", http.StatusUnauthorized)
			return
		}

		teamName := cookie.Value
		mu.Lock()
		_, ok := teams[teamName]
		mu.Unlock()

		if !ok {
			http.Error(w, "Невалиден отбор", http.StatusBadRequest)
			return
		}

		// Get the current quest status
		var quest Quest
		if err := db.Where("team_name = ? AND completed = ?", teamName, false).Order("quest_number asc").First(&quest).Error; err != nil {
			http.Error(w, "Няма текуща задача", http.StatusNotFound)
			return
		}

		// Prepare the response data
		status := map[string]interface{}{
			"questNumber":         quest.QuestNumber,
			"completed":           quest.Completed,
			"skipped":             quest.Skipped,
			"questTimerRunning":   quest.QuestTimerRunning,
			"questTimerEndTime":   quest.QuestTimerEndTime.Format(time.RFC3339),
			"questTimerFinished":  quest.QuestTimerFinished,
			"hintTimerRunning":    quest.HintTimerRunning,
			"hintTimerEndTime":    quest.HintTimerEndTime.Format(time.RFC3339),
			"hintTimerFinished":   quest.HintTimerFinished,
			"hintTimerRemaining":  time.Until(quest.HintTimerEndTime).String(),
			"questTimerRemaining": time.Until(quest.QuestTimerEndTime).String(),
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
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
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        1,
		// 	Text:               "В Плик №1 разполагате с предмети, които ще ви насочат коя е локацията, към която да поемете. В допълнение към тях, за да се ориентирате за името на тази забележителност,  получавате и тази анаграма: гарланех халими.",
		// 	CorrectAnswers:     "църква свети архангел михаил|свети архангел михаил|св. архангел михаил|архангел михаил",
		// 	Hint:               "Църквата е с име на главния архангел, главния пазител на небесното царство и главен страж на Божия закон, който превежда душите на мъртвите до ада или рая. ",
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  true,
		// 	HintTimerDuration:  2 * time.Second, // make to minutes
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        2,
		// 	Text:               "Когато пристигнете в църквата, се снимайте пред входа като протегнете длан напред и я сложите върху дланта на останалите.",
		// 	CorrectAnswers:     "",
		// 	FileRequired:       true,
		// 	QuestTimerRequired: false,
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        3,
		// 	Text:               "Легендата разказва, че църквата е построена през XII век. За да благодарят на Бога за подкрепата в успешната битка през 1190 г. в Тревненския проход, братята Асеневци построили три църкви, посветени на Св. Архангел Михаил. Едната от тях била в Трявна. Тя била опожарена при голямото кърджалийско нападение над Трявна през 1798 г. После тревненци се съвзели, ремонтирали църквата си и подновили служението. \nВлезте в църквата и запалете свещичката, с която разполагате (има по свещ за всеки).\n Timer-ът вече отброява 10 минутки от началото на quest-а, за да имате време за себе си в църквата. Ще можете да продължите нататък с quest-а след като минат 10-те минути. Когато времето изтече се съберете в двора на Църквата, направете снимка на цвете от двора на църквата и я изпратете.",
		// 	CorrectAnswers:     "",
		// 	FileRequired:       true,
		// 	QuestTimerRequired: true,
		// 	QuestTimerDuration: 10 * time.Second, // make to minutes
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        4,
		// 	Text:               "Разполагате с аудио, което да ви насочи към забележителността, до която трябва да стигнете. След като отговорите на quest-а стигнете до тази локация",
		// 	CorrectAnswers:     "часовникова кула|часовниковата кула|часовниковата кула в трявна|часовникова кула трявна|часовниковата кула трявна",
		// 	AudioPath:          "/static/audio/clockTowerBells.mp3",
		// 	QuestTimerRequired: false,
		// }, {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        5,
		// 	Text:               "Помолете минувач да ви снима пред Часовниковата кула като по най-оригинален начин се направете на часовници, часовникови механизми, махала, стрелки, циферблат, числа и т.н. ",
		// 	CorrectAnswers:     "",
		// 	FileRequired:       true,
		// 	QuestTimerRequired: false,
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        6,
		// 	Text:               "Век и половина след построяването на кулата към часовниковия механизъм е добавен магнетофон, благодарение на който всяка вечер точно в 22 ч. зазвучава песента по стихотворението „Неразделни“ на Пенчо Славейков.\nКои са основните герои в песента по текст на стихотворението “Неразделни”? ",
		// 	CorrectAnswers:     "калина и явор|калина, явор|калина,явор|явор и калина|явор, калина|явор,калина",
		// 	Hint:               "Разполагате с кратко аудио на песента.",
		// 	AudioPath:          "/static/audio/nerazdelni.mp3",
		// 	FileRequired:       false,
		// 	QuestTimerRequired: false,
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        7,
		// 	Text:               "Разберете от коя забележителност е тази снимка (например питайте хората от Трявна). Как се казва тази забележителност?",
		// 	CorrectAnswers:     "старото школо|старата школа|старата школа трявна|старото школо трявна",
		// 	Hint:               "Името на тази забележителност в превод на съвременен български език би било: “Старото училище”, но в миналото думата училище е била заместена с друга дума, която е означава същото. ",
		// 	ImagePath:          "/static/img/sh.png",
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  true,
		// 	HintTimerDuration:  2 * time.Second, // make to minutes
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        8,
		// 	Text:               "Старото школо в Трявна е едно от първите български светски училища, построено през 1839 г., в мрачните времена на османското иго.\nКогато пристигнете в Старото школо, намерете стаята на Класното училище (на втория етаж) и се снимайте седнали на банките като ученици, хванали перата. Някой от вас може да влезе в ролята на строг учител. Направете снимката максимално оригинална.",
		// 	CorrectAnswers:     "",
		// 	FileRequired:       true,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  false,
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        9,
		// 	Text:               "На ученическите банки ще откриете пясък, на който децата са пишели. Напишете нещо вдъхновяващо и го изпратете като снимка. ",
		// 	CorrectAnswers:     "",
		// 	FileRequired:       true,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  false,
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        10,
		// 	Text:               "В Класната стая влезте в ролите на учител и ученици. Нека част от вас застанат от ляво на учителската банка, пред черните табелки, с които са назидавали провинилите се учениците, а друга част - от дясно пред белите табелки, с които са поощрявали прилежните ученици. Пресъздайте емоциите на всеки от участниците в подобна реална ситуация и се снимайте. 🙂 ",
		// 	CorrectAnswers:     "",
		// 	FileRequired:       true,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  false,
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        11,
		// 	Text:               "На втория етаж, зала 2, открийте предмет с марка на известен съвременен автомобил. Какъв е този предмет? ",
		// 	CorrectAnswers:     "пишеща машина",
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  false,
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        12,
		// 	Text:               "На втория етаж, зала 2, има 4 вида времеизмервателни уреди. Напишете определението на всеки от тях в азбучен ред.",
		// 	CorrectAnswers:     "воден, огнен, огнено-маслен, пясъчен|воден,огнен,огнено-маслен,пясъчен",
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  false,
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        13,
		// 	Text:               "На втория етаж в ескпозицията със сметалото, изпишете една произволна дума с наличните букви и се снимайте на фона на сметалото, с което по оригинален начин изпишете 1,563,345. Можете да си направите селфи.",
		// 	CorrectAnswers:     "",
		// 	FileRequired:       true,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  false,
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        14,
		// 	Text:               "На втория етаж снимайте книгата, чието име “мирише на море”.",
		// 	CorrectAnswers:     "рибен буквар|рибният буквар|рибния буквар|буквар",
		// 	Hint:               "Написана е от Петър Берон и името й се състои от две думи: първата е морско/ водно животно, а втората е първата книга на децата в училище. ",
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  true,
		// 	HintTimerDuration:  2 * time.Second, // make to minutes
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        15,
		// 	Text:               "През 1845 г. в тревненското Старо школо учител става Петко Славейков. По негово време настъпили много промени в учебния процес и се поставило началото на ново развитие на учебното дело в града. Именно той въвел класното образование и нови предмети като пеене по ноти, рисуване, гимнастика, естествени науки и не на последно място писмено и говоримо турски, гръцки и френски език. Той целял българчетата да получават достойно на европейското образование. \nКое е мотото, което Петко Славейков ни е завещал, зашифровано в текстът по-долу: ",
		// 	CorrectAnswers:     "просвещението е нужно на всякой народ|просвещение е нужно на всякой народ",
		// 	ImagePath:          "/static/img/azbuka.png",
		// 	Hint:               "Можете да намерите мотото на стената в класното училище. ",
		// 	FileRequired:       false,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  true,
		// 	HintTimerDuration:  2 * time.Second, // make to minutes
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        16,
		// 	Text:               "Наредете пъзела в плик 2 и стигнете до забележителността, която ще откриете на него, когато го наредите. \nСнимайте готовия пъзел и го изпратете. ",
		// 	CorrectAnswers:     "",
		// 	FileRequired:       true,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  false,
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        17,
		// 	Text:               "На моста са останали следи от влюбени, които са се вричали в любов един на друг. \nКолко такива символа откриване на моста?",
		// 	CorrectAnswers:     "7|7 катинара|7 катинарчета",
		// 	FileRequired:       false,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  false,
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        18,
		// 	Text:               "Кивгиреният мост e стрoeн e прeз 1844 - 1845г. Първoнaчaлнo бил oт дървo, нo cлeд пoрoйни дъждoвe рeкaтa прииждaлa и чecтo гo cъбaрялa. Зaтoвa трeвнeнци рeшили дa гo пocтрoят oт кaмък. Мaйcтoр Димитър Ceргюв нaпрaвил мocтa в римcки cтил – виcoк, cвoдecт и cилнo изгърбeн дa мoжe cвoбoднo дa прoпуcкa придoшлитe буйни вoди нa рeкa Трeвнeнcкa. \nВъпрос: Как наричат още този мост тревненци? ",
		// 	CorrectAnswers:     "гърбавия мост|гърбав мост|гърбав|гърбавият мост",
		// 	Hint:               "Едно от определенията на моста по горе съдържа неговото име. Ако не успеете да се сетите, спрете минувач или влезте в близко дюкянче и попитайте тревненец.",
		// 	FileRequired:       false,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  true,
		// 	HintTimerDuration:  2 * time.Second, // make to minutes
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        19,
		// 	Text:               "По улица Петко Славейков между номер 21 и номер 17 има два Нречи Ванагри.  Открийте ги и ги снимайте.",
		// 	Hint:               "Махнете една буква от английската дума Crown и ще получите името на птицата, която търсите. ",
		// 	CorrectAnswers:     "",
		// 	FileRequired:       true,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  true,
		// 	HintTimerDuration:  2 * time.Second, // make to minutes
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        20,
		// 	Text:               "Къщата, до която трябва да стигнете носи името си от думата, която означава “учител” в миналото. \nВнимание: Имате 7-10 мин. ходене до тази локация. Изберете маршрутът ви да премине през ул. Петко Славейков. Това ще ви помогне за останалите quest-ове. ",
		// 	CorrectAnswers:     "даскаловата къща|даскалова къща",
		// 	FileRequired:       false,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  false,
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        21,
		// 	Text:               "“Тревненската колона” е дървопластика от ствола на 208-годишен дъб, израснал в Странджа планина край село Българи, която е изработена от съвременни майстори дърворезбари по случай 200 годишнината на Даскаловата къща. \nКогато стигнете до Даскаловата къща, помолете друг посетител/ служителя в музея да ви снима пред Тревненската колона в двора на къщата. ",
		// 	CorrectAnswers:     "",
		// 	FileRequired:       true,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  false,
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        22,
		// 	Text:               "В “Тревненската колона” открийте дърворезбата на дърворезбаря Слави Златанов (за целта ще трябва да ползвате информация, която не се намира на самата колона, но е в близост до нея).  С някои от цифрите, с които разполагате, направете трибуквена дума, която носи различен смисъл, когато я прочетете отляво надясно и отдясно наляво. ",
		// 	Hint:               "Прочетена отпред назад думата е синоним на “погледни!”",
		// 	CorrectAnswers:     "виж|жив|виж, жив|жив, виж|вижжив|живвиж|жив,виж|виж,жив",
		// 	FileRequired:       false,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  true,
		// 	HintTimerDuration:  2 * time.Second, // make to minutes
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        23,
		// 	Text:               "На първия етаж сред инструмените за дърворезба ще окриете кирпиден. \nСнимайте го и го изпратете. ",
		// 	Hint:               "С инструмент, изглеждащ по подобен начин се вадят зъби. ”",
		// 	FileRequired:       true,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  true,
		// 	HintTimerDuration:  2 * time.Second, // make to minutes
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        24,
		// 	Text:               "На първия етаж ще откриете Майсторско свидетелство за резбарство на Цани Тодоров Антонов от 15 декември 1931 г. \nКак се нарича Законът, съгласно който е издадено това свидетелство?",
		// 	Hint:               "С инструмент, изглеждащ по подобен начин се вадят зъби. ”",
		// 	CorrectAnswers:     "закон за организиране и подпомагане на занаятите|закон за организиране и подпомагане на занаяти|закона за организиране и подпомагане на занаятите|закона за организиране и подпомагане на занаяти|законът за организиране и подпомагане на занаяти|законът за организиране и подпомагане на занаятите",
		// 	FileRequired:       false,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  true,
		// 	HintTimerDuration:  2 * time.Second, // make to minutes
		// },
		// {
		// 	TeamName:           "TEAM1",
		// 	QuestNumber:        25,
		// 	Text:               "На първия етаж ще откриете Договор от 13 август 1938 г. между майстор резбар и църковното настоятелство за изработка на иконостас. \nВъпрос: От какъв материал според договора ще бъде изработена резбовската работа?",
		// 	Hint:               "Правим ароматен чай от цветовете на това дърво.",
		// 	CorrectAnswers:     "липов материал|липа|липов|липово дърво|липово",
		// 	FileRequired:       false,
		// 	QuestTimerRequired: false,
		// 	HintTimerRequired:  true,
		// 	HintTimerDuration:  2 * time.Second, // make to minutes
		// },
		{
			TeamName:           "TEAM1",
			QuestNumber:        26,
			Text:               "Качете се на втория етаж. В Патриотическата стая си направете обща снимка като застанете до някои от следните български ханове/ царе: Хан Аспарух, Цар Борис, Цар Самуил, Цар Калоян, Цар Иван Асен II, Цар Михаил Шишман\nПресъздайта максимално точно позицията, в който те стоят като използвате предмети, с които разполагате, за да покажете какво държат в ръцете си.\nПомолете друг посетител да ви снима, за да бъде снимката максимално автентична. ",
			CorrectAnswers:     "",
			FileRequired:       true,
			QuestTimerRequired: false,
			HintTimerRequired:  false,
		},
		{
			TeamName:           "TEAM1",
			QuestNumber:        27,
			Text:               "На втория етаж в Патриотическата стая открийте животното, което държи карта на България. \nВъпрос: Какво е това животно?",
			ImagePath:          "/static/img/lion.jpg",
			Hint:               "Прикачена снимка",
			CorrectAnswers:     "лъв|лъвът|лъвчето|лъвчетата|лъвове|лъвовете|лъвчета|лъвa",
			FileRequired:       false,
			QuestTimerRequired: false,
			HintTimerRequired:  true,
			HintTimerDuration:  2 * time.Second, // make to minutes
		},
		{
			TeamName:           "TEAM1",
			QuestNumber:        28,
			Text:               "На втория етаж ще откриете експозиция на занаят, свързан с обработка на сурова коприна, за декоративни цели - за нагръдници, пискюли на горни мъжки дрехи, пискюли на фесове, колани на жени и др. \nВъпрос: Как са наричали в миналото този занаят?",
			Hint:               "Думата се формира от следните букви „лъкасзак“",
			CorrectAnswers:     "казаслък",
			FileRequired:       false,
			QuestTimerRequired: false,
			HintTimerRequired:  true,
			HintTimerDuration:  2 * time.Second, // make to minutes
		},
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
