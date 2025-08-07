from flask import Flask, render_template, request, redirect, session, url_for, jsonify
from flask_pymongo import PyMongo
from bson.objectid import ObjectId
from datetime import datetime, timezone
import os
from dotenv import load_dotenv
from werkzeug.utils import secure_filename
import pathlib

load_dotenv()

app = Flask(__name__)
app.config['SECRET_KEY'] = os.getenv("SECRET_KEY")
app.config['MONGO_URI'] = os.getenv("MONGO_URI")

mongo = PyMongo(app)

# --- Global timer duration (in seconds) ---
GLOBAL_TIMER_DURATION = 2 * 60 * 60  # 2 hours

# --- Dynamically load teams from env ---
USERS = {}
for i in range(1, 5):  # Adjust if more teams
    user = os.getenv(f"TEAM{i}USER")
    pw = os.getenv(f"TEAM{i}PASS")
    if user and pw:
        USERS[user] = pw


# -------------------------
# LOGIN
# -------------------------
@app.route('/', methods=['GET', 'POST'])
def login():
    if request.method == 'POST':
        team = request.form.get('team')
        password = request.form.get('password')
        if USERS.get(team) == password:
            session['team_name'] = team

            # Ensure team doc exists
            team_doc = mongo.db.teams.find_one({"team_name": team})
            now_ts = int(datetime.now(timezone.utc).timestamp())

            if not team_doc:
                # Initialize with a default quest order (sorted by quest_number)
                quest_ids = [str(q["_id"]) for q in mongo.db.quests.find().sort("quest_number", 1)]
                mongo.db.teams.insert_one({
                    "team_name": team,
                    "quest_order": quest_ids,
                    "current_quest_idx": 0,
                    "quest_progress": {},
                    "global_timer_start": now_ts
                })
            else:
                # Ensure global timer exists for old teams
                if "global_timer_start" not in team_doc:
                    mongo.db.teams.update_one(
                        {"_id": team_doc["_id"]},
                        {"$set": {"global_timer_start": now_ts}}
                    )

            return redirect(url_for('treasurehunt'))
        else:
            return render_template('login.html', error="Invalid credentials")
    return render_template('login.html')


# -------------------------
# TIME UP PAGE
# -------------------------
@app.route('/time-up')
def time_up():
    return "<h1>⏳ Time's up!</h1><p>Your 2 hours have expired.</p>"


# -------------------------
# TREASURE HUNT MAIN PAGE
# -------------------------
@app.route('/treasurehunt')
def treasurehunt():
    team_name = session.get('team_name')
    if not team_name:
        return redirect(url_for('login'))

    team_doc = mongo.db.teams.find_one({"team_name": team_name})
    if not team_doc:
        return redirect(url_for('logout'))

    now = int(datetime.now(timezone.utc).timestamp())

    # --- Check global timer expiration ---
    global_start = team_doc.get("global_timer_start")
    if global_start is None:
        global_start = now
        mongo.db.teams.update_one(
            {"_id": team_doc["_id"]},
            {"$set": {"global_timer_start": global_start}}
        )
    if now >= global_start + GLOBAL_TIMER_DURATION:
        return redirect(url_for('time_up'))

    # Get current quest ID
    current_idx = team_doc["current_quest_idx"]
    quest_id = team_doc["quest_order"][current_idx]
    quest = mongo.db.quests.find_one({"_id": ObjectId(quest_id)})

    # Current progress for this quest
    progress = team_doc.get("quest_progress", {}).get(quest_id, {})

    # Start timers if not set
    if "quest_timer_start" not in progress:
        progress["quest_timer_start"] = now
    if "hint_timer_start" not in progress:
        progress["hint_timer_start"] = now

    # Update in DB if new
    mongo.db.teams.update_one(
        {"_id": team_doc["_id"]},
        {"$set": {f"quest_progress.{quest_id}": progress}}
    )

    quest_timer_start = progress["quest_timer_start"]
    hint_timer_start = progress["hint_timer_start"]
    quest_timer_duration = quest.get('quest_timer_duration', 0)
    hint_timer_duration = quest.get('hint_timer_duration', 0)

    return render_template(
        "treasurehunt.html",
        quest=quest,
        quest_timer_duration=quest_timer_duration,
        quest_timer_start=quest_timer_start,
        hint_timer_duration=hint_timer_duration,
        hint_timer_start=hint_timer_start,
        global_timer_duration=GLOBAL_TIMER_DURATION,
        global_timer_start=global_start
    )


# -------------------------
# API: TIMERS
# -------------------------
@app.route('/api/timers')
def api_timers():
    team_name = session.get('team_name')
    if not team_name:
        return jsonify({"error": "Not logged in"}), 401

    team_doc = mongo.db.teams.find_one({"team_name": team_name})
    if not team_doc:
        return jsonify({"error": "Team not found"}), 404

    now = int(datetime.now(timezone.utc).timestamp())

    # --- Check global timer expiration ---
    global_start = team_doc.get("global_timer_start")
    if global_start is None:
        global_start = now
        mongo.db.teams.update_one(
            {"_id": team_doc["_id"]},
            {"$set": {"global_timer_start": global_start}}
        )
    if now >= global_start + GLOBAL_TIMER_DURATION:
        return jsonify({"error": "Time's up"}), 403

    current_idx = team_doc["current_quest_idx"]
    quest_id = team_doc["quest_order"][current_idx]
    quest = mongo.db.quests.find_one({"_id": ObjectId(quest_id)})

    progress = team_doc.get("quest_progress", {}).get(quest_id, {})
    if "quest_timer_start" not in progress:
        progress["quest_timer_start"] = now
    if "hint_timer_start" not in progress:
        progress["hint_timer_start"] = now

    mongo.db.teams.update_one(
        {"_id": team_doc["_id"]},
        {"$set": {f"quest_progress.{quest_id}": progress}}
    )

    return jsonify({
        "questTimerDuration": quest.get('quest_timer_duration', 0),
        "questTimerStart": progress["quest_timer_start"],
        "hintTimerDuration": quest.get('hint_timer_duration', 0),
        "hintTimerStart": progress["hint_timer_start"],
        "globalTimerDuration": GLOBAL_TIMER_DURATION,
        "globalTimerStart": global_start
    })


# -------------------------
# SUBMIT ANSWER
# -------------------------
@app.route('/submit', methods=['POST'])
def submit():
    team_name = session.get('team_name')
    if not team_name:
        return redirect(url_for('login'))

    team_doc = mongo.db.teams.find_one({"team_name": team_name})
    if not team_doc:
        return redirect(url_for('logout'))

    now = int(datetime.now(timezone.utc).timestamp())

    # --- Block submissions if global timer expired ---
    global_start = team_doc.get("global_timer_start", now)
    if now >= global_start + GLOBAL_TIMER_DURATION:
        return redirect(url_for('time_up'))

    current_idx = team_doc["current_quest_idx"]
    quest_id = team_doc["quest_order"][current_idx]
    quest = mongo.db.quests.find_one({"_id": ObjectId(quest_id)})

    answer = request.form.get('answer', '').strip().lower()
    correct_answers = [a.strip().lower() for a in quest.get("correct_answers", "").split("|") if a.strip()]

    # --- Handle file upload ---
    uploaded_file = request.files.get("uploaded_file")
    file_uploaded = False
    if uploaded_file and uploaded_file.filename.strip():
        safe_name = secure_filename(uploaded_file.filename)
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")

        # Extract team number (team1 -> 1)
        team_number = ''.join(filter(str.isdigit, team_name)) or "unknown"

        # Create output folder
        save_dir = pathlib.Path("output") / f"team_{team_number}"
        save_dir.mkdir(parents=True, exist_ok=True)

        # Full file path
        file_path = save_dir / f"{timestamp}_{safe_name}"

        uploaded_file.save(file_path)
        print(f"[UPLOAD] Saved file for {team_name} to {file_path}")

        file_uploaded = True

        # Save file info in DB
        mongo.db.teams.update_one(
            {"_id": team_doc["_id"]},
            {"$push": {f"quest_progress.{quest_id}.uploaded_files": str(file_path)}}
        )

    # --- Determine completion logic ---
    completed = False
    if correct_answers:
        if answer and answer in correct_answers:
            completed = True
    else:
        # If no answer required, complete if file is required and uploaded
        if quest.get("file_required") and file_uploaded:
            completed = True

    # Save progress info
    mongo.db.teams.update_one(
        {"_id": team_doc["_id"]},
        {"$set": {
            f"quest_progress.{quest_id}.submitted_answer": answer,
            f"quest_progress.{quest_id}.completed": completed
        }}
    )

    # If completed, go to next quest
    if completed:
        if current_idx + 1 < len(team_doc["quest_order"]):
            mongo.db.teams.update_one(
                {"_id": team_doc["_id"]},
                {"$set": {"current_quest_idx": current_idx + 1}}
            )
            return redirect(url_for('treasurehunt'))
        else:
            return "🎉 All quests completed!"

    return redirect(url_for('treasurehunt'))


# -------------------------
# LOGOUT
# -------------------------
@app.route('/logout')
def logout():
    session.clear()
    return redirect(url_for('login'))


if __name__ == '__main__':
    app.run(host="0.0.0.0", port=9000, debug=True)
