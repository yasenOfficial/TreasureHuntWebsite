from flask import Flask, render_template, request, redirect, session, url_for, jsonify, abort
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
# TIME UP PAGE (kept but redirects to gamefinished)
# -------------------------
@app.route('/time-up')
def time_up():
    # Keep the route for compatibility, but unify UI at /gamefinished
    return redirect(url_for('gamefinished', status='timeup'))


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
        return redirect(url_for('gamefinished', status='timeup'))

    # Current quest
    current_idx = team_doc["current_quest_idx"]
    quest_id = team_doc["quest_order"][current_idx]
    quest = mongo.db.quests.find_one({"_id": ObjectId(quest_id)})
    
    phase_answers = []
    if quest.get("list_phase_answers") is not None:
        target_phase = quest["list_phase_answers"]
        phase_quests = mongo.db.quests.find({"phase": target_phase})
        for pq in phase_quests:
            ans = pq.get("correct_answers", "")
            if ans:
                first_answer = ans.split("|")[0].strip()
                if first_answer:
                    phase_answers.append(first_answer)

    # Remove duplicates while preserving order
    phase_answers = list(dict.fromkeys(phase_answers))



    # Progress for this quest
    progress = team_doc.get("quest_progress", {}).get(quest_id, {})
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
        global_timer_start=global_start,
        phase_answers=phase_answers   # <-- new
    )



# -------------------------
# API: TIMERS (for JS)
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

    # Global timer
    global_start = team_doc.get("global_timer_start")
    if global_start is None:
        global_start = now
        mongo.db.teams.update_one(
            {"_id": team_doc["_id"]},
            {"$set": {"global_timer_start": global_start}}
        )
    if now >= global_start + GLOBAL_TIMER_DURATION:
        return jsonify({
            "error": "Time's up",
            "redirect": url_for('gamefinished', status='timeup')
        }), 403

    # Current quest
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

@app.route("/api/use_hint", methods=["POST"])
def api_use_hint():
    team_name = session.get("team_name")
    if not team_name:
        return jsonify({"error": "Not logged in"}), 401

    team_doc = mongo.db.teams.find_one({"team_name": team_name})
    if not team_doc:
        return jsonify({"error": "Team not found"}), 404

    current_idx = team_doc["current_quest_idx"]
    quest_id = team_doc["quest_order"][current_idx]

    now_ts = int(datetime.now(timezone.utc).timestamp())

    mongo.db.teams.update_one(
        {"_id": team_doc["_id"]},
        {"$set": {
            f"quest_progress.{quest_id}.used_hint": True,
            f"quest_progress.{quest_id}.hint_revealed_at": now_ts
        }}
    )

    return jsonify({"status": "ok"})


# -------------------------
# SUBMIT (supports grid cipher)
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
    global_start = team_doc.get("global_timer_start", now)
    if now >= global_start + GLOBAL_TIMER_DURATION:
        return redirect(url_for('gamefinished', status='timeup'))

    current_idx = team_doc["current_quest_idx"]
    quest_id = team_doc["quest_order"][current_idx]
    quest = mongo.db.quests.find_one({"_id": ObjectId(quest_id)})

    # Answers
    answer = (request.form.get('answer') or '').strip().lower()
    grid_cipher_raw = (request.form.get('grid_cipher') or '').strip()

    # Simple text answers list
    correct_answers = [a.strip().lower()
                       for a in (quest.get("correct_answers", "") or "").split("|")
                       if a.strip()]

    # File upload
    uploaded_file = request.files.get("uploaded_file")
    file_uploaded = False
    if uploaded_file and uploaded_file.filename.strip():
        safe_name = secure_filename(uploaded_file.filename)
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        team_number = ''.join(filter(str.isdigit, team_name)) or "unknown"
        save_dir = pathlib.Path("output") / f"team_{team_number}"
        save_dir.mkdir(parents=True, exist_ok=True)
        file_path = save_dir / f"{timestamp}_{safe_name}"
        uploaded_file.save(file_path)
        file_uploaded = True

        mongo.db.teams.update_one(
            {"_id": team_doc["_id"]},
            {"$push": {f"quest_progress.{quest_id}.uploaded_files": str(file_path)}}
        )

    # Grid helpers
    def only_digits(s: str) -> str:
        return ''.join(ch for ch in s if ch.isdigit())

    # Determine if quest has a grid like "4x4"
    has_grid = bool(quest.get("grid"))
    grid_matrix = None
    grid_ok = False

    if has_grid:
        # normalize incoming cipher (should already be digits from gridBuilder)
        gc = only_digits(grid_cipher_raw)
        # Parse grid dims
        try:
            rows, cols = map(int, str(quest["grid"]).lower().split("x"))
        except Exception:
            rows, cols = 0, 0

        # If we got the right length, convert to matrix and check
        if rows > 0 and cols > 0 and len(gc) == rows * cols:
            # Save in DB as 2D array
            grid_matrix = [
                [int(gc[r * cols + c]) for c in range(cols)]
                for r in range(rows)
            ]

            # Build expected string row-major
            expected = None
            if quest.get("grid_answer"):
                expected = only_digits(str(quest["grid_answer"]))
            elif quest.get("grid_values"):
                try:
                    flat = ''.join(''.join(str(x) for x in row) for row in quest["grid_values"])
                    expected = only_digits(flat)
                except Exception:
                    expected = None
            # if there's also correct_answers, allow matching there too
            candidates = [expected] if expected else []
            candidates += correct_answers

            if candidates:
                grid_ok = gc in candidates
            else:
                # If no explicit answers are defined, accept any full grid submission
                grid_ok = True

    # Completion rules
    completed = False
    if has_grid:
        completed = grid_ok
    else:
        if correct_answers:
            completed = bool(answer) and (answer in correct_answers)
        else:
            # no answer required → complete when file is required & uploaded
            if quest.get("file_required"):
                completed = file_uploaded

    # Persist progress
    update_doc = {
        f"quest_progress.{quest_id}.submitted_answer": answer,
        f"quest_progress.{quest_id}.completed": completed
    }
    # store grid_matrix (2D)
    if has_grid:
        update_doc[f"quest_progress.{quest_id}.grid_matrix"] = grid_matrix

    mongo.db.teams.update_one(
        {"_id": team_doc["_id"]},
        {"$set": update_doc}
    )

    # Next step or toast
    if completed:
        # COMENTED OUT ONLY FOR DEBBUING THIS TO NOT GO TO NEXT QUEST TO BE FASTER

        if current_idx + 1 < len(team_doc["quest_order"]):
            mongo.db.teams.update_one(
                {"_id": team_doc["_id"]},
                {"$set": {"current_quest_idx": current_idx + 1}}
            )
            return redirect(url_for('treasurehunt', status="success"))
        else:
            return redirect(url_for('gamefinished'))
    else:
        return redirect(url_for('treasurehunt', status="wrong"))


# -------------------------
# SKIP QUEST
# -------------------------
@app.route('/skip', methods=['POST'])
def skip():
    team_name = session.get('team_name')
    if not team_name:
        return redirect(url_for('login'))

    team_doc = mongo.db.teams.find_one({"team_name": team_name})
    if not team_doc:
        return redirect(url_for('logout'))

    now = int(datetime.now(timezone.utc).timestamp())
    global_start = team_doc.get("global_timer_start", now)
    if now >= global_start + GLOBAL_TIMER_DURATION:
        return redirect(url_for('gamefinished', status='timeup'))

    current_idx = team_doc["current_quest_idx"]
    quest_id = team_doc["quest_order"][current_idx]

    mongo.db.teams.update_one(
        {"_id": team_doc["_id"]},
        {"$set": {
            f"quest_progress.{quest_id}.skipped": True,
            f"quest_progress.{quest_id}.completed": True
        }}
    )

    # COMENTED OUT ONLY FOR DEBBUING THIS TO NOT GO TO NEXT QUEST TO BE FASTER

    if current_idx + 1 < len(team_doc["quest_order"]):
        mongo.db.teams.update_one(
            {"_id": team_doc["_id"]},
            {"$set": {"current_quest_idx": current_idx + 1}}
        )
        return redirect(url_for('treasurehunt', status="skip"))
    else:
        return redirect(url_for('gamefinished', status="skip"))


# -------------------------
# GAME FINISHED (scoring + banner)
# -------------------------
@app.route("/gamefinished", endpoint="gamefinished")
def game_finished():
    team_name = session.get("team_name") or request.args.get("team")
    if not team_name:
        abort(400, description="Missing team. Pass ?team=team_name or set it in session.")

    status = request.args.get("status")  # <-- read the end reason from querystring

    team = mongo.db.teams.find_one({"team_name": team_name})
    if not team:
        abort(404, description="Team not found")

    quest_order = team.get("quest_order", [])
    progress_map = team.get("quest_progress", {})

    solved_no_hint = 0
    solved_with_hint = 0
    skipped_count = 0
    total_points = 0
    rows = []

    for idx, qid in enumerate(quest_order, start=1):
        quest = mongo.db.quests.find_one({"_id": ObjectId(qid)})
        quest_number = quest.get("quest_number", idx) if quest else idx

        pg = progress_map.get(qid, {}) or {}
        skipped = bool(pg.get("skipped", False))
        completed = bool(pg.get("completed", False))
        used_hint = bool(
            pg.get("used_hint") or
            pg.get("hint_used") or
            pg.get("hint_shown") or
            (pg.get("hint_revealed_at") is not None)
        )

        if skipped:
            status_text = "Skipped"
            points = 0
            skipped_count += 1
        elif completed:
            if used_hint:
                status_text = "Solved (with hint)"
                points = 3
                solved_with_hint += 1
            else:
                status_text = "Solved (no hint)"
                points = 5
                solved_no_hint += 1
        else:
            status_text = "In progress"
            points = 0

        total_points += points
        rows.append({
            "quest_number": quest_number,
            "status": status_text,
            "used_hint": used_hint,
            "points": points
        })

    totals = {
        "team_name": team_name,
        "solved_no_hint": solved_no_hint,
        "solved_with_hint": solved_with_hint,
        "skipped": skipped_count,
        "correct": solved_no_hint + solved_with_hint,
        "points": total_points
    }

    return render_template("gamefinished.html", totals=totals, rows=rows, end_reason=status)


# -------------------------
# LOGOUT
# -------------------------
@app.route('/logout')
def logout():
    session.clear()
    return redirect(url_for('login'))


if __name__ == '__main__':
    app.run(host="0.0.0.0", port=9000, debug=True)
