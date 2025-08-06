# Treasure Hunt Website

## Features

- **Skip Functionality:** Teams can skip a quest by typing "SKIP" as their answer. This will be logged and the team will automatically move to the next quest.
- **Hint Usage:** Teams can view hints by clicking the "Show Hint" button. Each hint usage is logged.

## Logs

- **Skip Log:** The application tracks how many quests a team has skipped. This information is stored in a log file named after the team (e.g., TEAM1_log.txt).
- **Hint Log:** The application tracks how many hints each team has used. This information is also stored in the team's log file.

## Log Files

Each team has a log file (e.g., `TEAM1_log.txt`). The log file records:

- The date and time a hint was used.
- The date and time a quest was skipped.
