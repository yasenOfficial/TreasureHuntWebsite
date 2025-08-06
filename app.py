import os
import datetime
from flask import Flask, render_template, request, redirect, jsonify, make_response
from flask_jwt_extended import (
    JWTManager, create_access_token, create_refresh_token, jwt_required,
    get_jwt_identity, set_refresh_cookies, unset_jwt_cookies
)

from pymongo import MongoClient
from dotenv import load_dotenv

load_dotenv()

app = Flask(__name__)
app.config['SECRET_KEY'] = os.getenv('SECRET_KEY')
app.config["JWT_SECRET_KEY"] = os.getenv("JWT_SECRET")
app.config["JWT_TOKEN_LOCATION"] = ["cookies"]
app.config["JWT_ACCESS_COOKIE_PATH"] = "/"
app.config["JWT_REFRESH_COOKIE_PATH"] = "/token/refresh"
app.config["JWT_COOKIE_SECURE"] = False  # True if HTTPS
app.config["JWT_ACCESS_TOKEN_EXPIRES"] = datetime.timedelta(minutes=15)
app.config["JWT_REFRESH_TOKEN_EXPIRES"] = datetime.timedelta(days=7)

jwt = JWTManager(app)

client = MongoClient(os.getenv("MONGO_URI"))
db = client["treasurehunt"]
quests_collection = db["quests"]

# Team credentials from .env
TEAMS = {
    "TEAM1": {"username": os.getenv("TEAM1USER"), "password": os.getenv("TEAM1PASS")},
    "TEAM2": {"username": os.getenv("TEAM2USER"), "password": os.getenv("TEAM2PASS")},
    "TEAM3": {"username": os.getenv("TEAM3USER"), "password": os.getenv("TEAM3PASS")},
    "TEAM4": {"username": os.getenv("TEAM4USER"), "password": os.getenv("TEAM4PASS")},
}


@app.route("/", methods=["GET"])
def index():
    return render_template("index.html", message="Моля, въведете вашите данни за вход")


@app.route("/login", methods=["POST"])
def login():
    username = request.form.get("username")
    password = request.form.get("password")
    for team_name, creds in TEAMS.items():
        if creds["username"] == username and creds["password"] == password:
            # Issue tokens
            access_token = create_access_token(identity=team_name)
            refresh_token = create_refresh_token(identity=team_name)

            resp = make_response(redirect(f"/treasurehunt"))
            # Set JWT cookies
            resp.set_cookie("access_token_cookie", access_token, httponly=True, max_age=15*60)
            set_refresh_cookies(resp, refresh_token)
            return resp
    # Failed login
    return render_template("index.html", message="Невалидни данни за вход"), 401


@app.route("/treasurehunt")
@jwt_required()
def treasurehunt():
    team_name = get_jwt_identity()
    # Find the next incomplete quest for the team
    quest = quests_collection.find_one({"team_name": team_name, "completed": False}, sort=[("quest_number", 1)])
    if not quest:
        return redirect(f"/gamefinished")
    return render_template("treasurehunt.html", quest=quest, team=team_name)


@app.route("/submit", methods=["POST"])
@jwt_required()
def submit():
    team_name = get_jwt_identity()
    quest_id = request.form.get("quest_id")
    answer = request.form.get("answer", "")

    quest = quests_collection.find_one({"_id": quest_id, "team_name": team_name})
    if not quest:
        return "Quest not found", 404

    correct_answers = [a.strip().lower() for a in quest["correct_answers"].split("|")]
    is_correct = answer.strip().lower() in correct_answers

    if answer.strip().lower() == "skip":
        quests_collection.update_one({"_id": quest_id}, {"$set": {"skipped": True, "completed": True}})
        return redirect("/treasurehunt?skipped=true")

    if is_correct:
        quests_collection.update_one({"_id": quest_id}, {"$set": {"completed": True}})
        return redirect("/treasurehunt?success=true")
    else:
        return redirect("/treasurehunt?success=false")


@app.route("/logout")
def logout():
    resp = make_response(redirect("/"))
    unset_jwt_cookies(resp)
    resp.set_cookie("access_token_cookie", "", expires=0)
    return resp

@app.route("/token/refresh", methods=["POST"])
@jwt_required(refresh=True)   # ✅ Use this!
def refresh():
    team_name = get_jwt_identity()
    access_token = create_access_token(identity=team_name)
    resp = jsonify({"msg": "Token refreshed"})
    resp.set_cookie("access_token_cookie", access_token, httponly=True, max_age=15*60)
    return resp

@app.route("/gamefinished")
@jwt_required()
def gamefinished():
    team_name = get_jwt_identity()
    # You can aggregate skip/hint/complete stats here
    total = quests_collection.count_documents({"team_name": team_name})
    completed = quests_collection.count_documents({"team_name": team_name, "completed": True})
    skipped = quests_collection.count_documents({"team_name": team_name, "skipped": True})
    hints = sum([q.get("hints_used", 0) for q in quests_collection.find({"team_name": team_name})])
    return render_template("gamefinished.html", total=total, completed=completed, skipped=skipped, hints=hints, team=team_name)

# --- Utility: Create index.html mockup if you want!
# You must create templates/index.html, templates/treasurehunt.html, templates/gamefinished.html

if __name__ == "__main__":
    app.run(port=8080, debug=True)
