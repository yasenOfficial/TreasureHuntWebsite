package main

import (
	"encoding/csv"
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
	seedDatabaseFromCSV(db, "data/quests.csv")

	// Parse templates once and cache them
	templates = template.Must(template.ParseGlob(fmt.Sprintf("%s/*.html", templateDir)))
}

func main() {
	defer db.Close()

	// Initialize teams with credentials from environment variables
	teams["TEAM1"] = &Team{Username: os.Getenv("TEAM1USER"), Password: os.Getenv("TEAM1PASS")}
	teams["TEAM2"] = &Team{Username: os.Getenv("TEAM2USER"), Password: os.Getenv("TEAM2PASS")}
	teams["TEAM3"] = &Team{Username: os.Getenv("TEAM3USER"), Password: os.Getenv("TEAM3PASS")}
	teams["TEAM4"] = &Team{Username: os.Getenv("TEAM4USER"), Password: os.Getenv("TEAM4PASS")}

	// Serve static files
	http.Handle(
		"/static/css/",
		http.StripPrefix("/static/css/", http.FileServer(http.Dir(fmt.Sprintf("%s/static/css", templateDir)))),
	)
	http.Handle(
		"/static/js/",
		http.StripPrefix("/static/js/", http.FileServer(http.Dir(fmt.Sprintf("%s/static/js", templateDir)))),
	)
	http.Handle(
		"/static/img/",
		http.StripPrefix("/static/img/", http.FileServer(http.Dir(fmt.Sprintf("%s/static/img", templateDir)))),
	)
	http.Handle(
		"/static/audio/",
		http.StripPrefix("/static/audio/", http.FileServer(http.Dir(fmt.Sprintf("%s/static/audio", templateDir)))),
	)

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
			// http.Error(w, "Невалидни данни за вход", http.StatusUnauthorized)
			http.Redirect(w, r, "/", http.StatusSeeOther)
		} else {
			// http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			// http.Error(w, "Невалидни данни за вход", http.StatusUnauthorized)
			http.Redirect(w, r, "/", http.StatusSeeOther)

		}
	})

	http.HandleFunc("/treasurehunt", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("logged_in_team")
		if err != nil {
			// http.Error(w, "Unauthorized", http.StatusUnauthorized)
			// http.Error(w, "Неоторизиран достъп", http.StatusUnauthorized)
			http.Redirect(w, r, "/", http.StatusSeeOther)

			return
		}

		teamName := cookie.Value
		requestedTeam := r.URL.Query().Get("team")
		success := r.URL.Query().Get("success")
		skipped := r.URL.Query().Get("skipped")

		if teamName != requestedTeam {
			// http.Error(w, "Unauthorized", http.StatusUnauthorized)
			// http.Error(w, "Неоторизиран достъп", http.StatusUnauthorized)
			http.Redirect(w, r, "/", http.StatusSeeOther)

			return
		}

		mu.Lock()
		team, ok := teams[teamName]
		mu.Unlock()

		if !ok {
			// http.Error(w, "Invalid team", http.StatusBadRequest)
			// http.Error(w, "Невалиден отбор", http.StatusBadRequest)
			http.Redirect(w, r, "/", http.StatusSeeOther)

			return
		}

		if team.GameFinished {
			// Count the number of skipped quests
			var skipCount int64
			db.Model(&Quest{}).Where("team_name = ? AND skipped = ?", teamName, true).Count(&skipCount)

			// Count the number of hints used
			var hintCount int64
			db.Model(&Quest{}).Where("team_name = ?", teamName).Select("sum(hints_used)").Row().Scan(&hintCount)

			// Count the number of completed quests
			var completedCount int64
			db.Model(&Quest{}).Where("team_name = ? AND completed = ?", teamName, true).Count(&completedCount)
			completedCount = completedCount - skipCount

			// Redirect to the game finished page with hint count, skip count, and completed quests
			http.Redirect(
				w,
				r,
				fmt.Sprintf(
					"/gamefinished?team=%s&hintCount=%d&skipCount=%d&questsCompleted=%d",
					teamName,
					hintCount,
					skipCount,
					completedCount,
				),
				http.StatusSeeOther,
			)
			return
		}

		elapsed := time.Since(team.Stopwatch)

		// Get the total number of quests
		var totalQuests int64
		db.Model(&Quest{}).Where("team_name = ?", teamName).Count(&totalQuests)

		// Get the current quest
		var quest Quest
		if err := db.Where("team_name = ? AND completed = ?", teamName, false).Order("quest_number asc").First(&quest).Error; err != nil {
			// If no active quests, assume the game is finished and redirect to gamefinished

			// Count the number of skipped quests
			var skipCount int64
			db.Model(&Quest{}).Where("team_name = ? AND skipped = ?", teamName, true).Count(&skipCount)

			// Count the number of hints used
			var hintCount int64
			db.Model(&Quest{}).Where("team_name = ?", teamName).Select("sum(hints_used)").Row().Scan(&hintCount)

			// Count the number of completed quests
			var completedCount int64
			db.Model(&Quest{}).Where("team_name = ? AND completed = ?", teamName, true).Count(&completedCount)
			completedCount = completedCount - skipCount

			// Redirect to the game finished page with hint count, skip count, and completed quests
			http.Redirect(
				w,
				r,
				fmt.Sprintf(
					"/gamefinished?team=%s&hintCount=%d&skipCount=%d&questsCompleted=%d",
					teamName,
					hintCount,
					skipCount,
					completedCount,
				),
				http.StatusSeeOther,
			)
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
				// http.Error(w, "Неавторизиран достъп", http.StatusUnauthorized)
				http.Redirect(w, r, "/", http.StatusSeeOther)

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
				// http.Error(w, "Задачата не е намерена", http.StatusNotFound)
				http.Redirect(w, r, "/", http.StatusSeeOther)

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
			// http.Error(w, "Невалиден метод на заявка", http.StatusMethodNotAllowed)
			http.Redirect(w, r, "/", http.StatusSeeOther)

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
			// http.Error(w, "Неоторизиран достъп", http.StatusUnauthorized)
			http.Redirect(w, r, "/", http.StatusSeeOther)

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
			// Redirect to home if no team is specified
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Retrieve hint count from query parameters
		hintCount, err := strconv.ParseInt(r.URL.Query().Get("hintCount"), 10, 64)
		if err != nil {
			hintCount = 0
		}

		// Retrieve skip count from query parameters
		skipCount, err := strconv.ParseInt(r.URL.Query().Get("skipCount"), 10, 64)
		if err != nil {
			skipCount = 0
		}

		// Retrieve quests completed count (assuming it's provided in the query parameters)
		questsCompleted, err := strconv.ParseInt(r.URL.Query().Get("questsCompleted"), 10, 64)
		if err != nil {
			questsCompleted = 0
		}

		// Retrieve total quests count (assuming it's provided or can be calculated)
		var totalQuests int64
		db.Model(&Quest{}).Where("team_name = ?", teamName).Count(&totalQuests)

		// Prepare the data for rendering the template
		data := struct {
			HintCount       int64
			SkipCount       int64
			QuestsCompleted int64
			TotalQuests     int64
		}{
			HintCount:       hintCount,
			SkipCount:       skipCount,
			QuestsCompleted: questsCompleted,
			TotalQuests:     totalQuests,
		}

		// Log final stats to a file
		logFilePath := "teams_finished.log" // The path to the log file
		logEntry := fmt.Sprintf("Team: %s | Hints Used: %d | Skips: %d | Quests Completed: %d/%d\n",
			teamName, hintCount, skipCount, questsCompleted, totalQuests)

		mu.Lock() // Ensure thread-safe file writing
		file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Failed to open log file: %v", err)
		} else {
			defer file.Close()
			if _, err := file.WriteString(logEntry); err != nil {
				log.Printf("Failed to write to log file: %v", err)
			}
		}
		mu.Unlock()

		// Render the template with the final data
		err = templates.ExecuteTemplate(w, "gamefinished.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/check-quest-status", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("logged_in_team")
		if err != nil {
			// http.Error(w, "Неоторизиран достъп", http.StatusUnauthorized)
			http.Redirect(w, r, "/", http.StatusSeeOther)

			return
		}

		teamName := cookie.Value
		mu.Lock()
		_, ok := teams[teamName]
		mu.Unlock()

		if !ok {
			// http.Error(w, "Невалиден отбор", http.StatusBadRequest)
			http.Redirect(w, r, "/", http.StatusSeeOther)

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
				if team.StopwatchOn &&
					time.Since(
						team.Stopwatch,
					) >= 2*time.Hour { // FIX --------------------- THE TIME THE GAME WILL LAST --------------------------------------------
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

// Function to read quests from a CSV file and seed the database
func seedDatabaseFromCSV(db *gorm.DB, filePath string) {
	// Open the CSV file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening CSV file:", err)
		return
	}
	defer file.Close()

	// Create a new CSV reader
	reader := csv.NewReader(file)

	// Read all records from the CSV file
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error reading CSV file:", err)
		return
	}

	// Clear the database
	db.Exec("DELETE FROM quests")

	// Check if the quests are already in the database
	var count int64
	db.Model(&Quest{}).Count(&count)
	if count > 0 {
		fmt.Println("Database already seeded.")
		return
	}

	// Process the CSV records
	for _, record := range records {
		// Replace literal \n with actual newlines in text fields
		for i, field := range record {
			record[i] = strings.ReplaceAll(field, `\n`, "\n")
		}

		// Convert and map CSV fields to Quest struct
		quest := Quest{
			TeamName:           record[0],
			QuestNumber:        parseInt(record[1]),
			Text:               record[2],
			CorrectAnswers:     record[3],
			Hint:               record[4],
			AudioPath:          record[5],
			ImagePath:          record[6],
			FileRequired:       parseBool(record[7]),
			QuestTimerRequired: parseBool(record[8]),
			QuestTimerDuration: parseDuration(record[9]),
			HintTimerRequired:  parseBool(record[10]),
			HintTimerDuration:  parseDuration(record[11]),
		}

		// Insert quest into the database
		db.Create(&quest)
	}

	fmt.Println("Database seeded with quests from CSV.")
}

// Helper function to parse int
func parseInt(value string) int {
	v, _ := strconv.Atoi(value)
	return v
}

// Helper function to parse bool
func parseBool(value string) bool {
	v, _ := strconv.ParseBool(value)
	return v
}

// Helper function to parse duration
func parseDuration(value string) time.Duration {
	// Attempt to parse the string as a standard duration (e.g., "1h30m", "45s")
	duration, err := time.ParseDuration(value)
	if err == nil {
		return duration
	}

	// If parsing as a standard duration string fails, check if it's a simple number
	// with a time unit that may have been omitted.
	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		return time.Duration(seconds * float64(time.Second))
	}

	// If all parsing attempts fail, return a zero duration
	return 0
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
