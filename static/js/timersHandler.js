window.addEventListener('DOMContentLoaded', function() {
    console.log("DOMContentLoaded fired, fetching /api/timers...");

    function formatTime(ts) {
        if (!ts) return "N/A";
        return new Date(ts * 1000).toLocaleString();
    }

    fetch('/api/timers')
      .then(res => {
        console.log("/api/timers response status:", res.status);
        return res.json();
      })
      .then(data => {
        console.log("Timer data from /api/timers:", data);

        const questTimerDuration = data.questTimerDuration;
        const questTimerStart = data.questTimerStart;
        const hintTimerDuration = data.hintTimerDuration;
        const hintTimerStart = data.hintTimerStart;

        const submitBtn = document.getElementById('submit-btn');
        const hintBtn = document.getElementById('hint-btn');
        // You can ignore these now, or keep them for debugging
        // const questTimerElem = document.getElementById('quest-timer');
        // const hintTimerElem = document.getElementById('hint-timer');
        const hintContent = document.getElementById('hint-content');

        // --- QUEST TIMER ---
        function updateQuestTimer() {
            const now = Math.floor(Date.now() / 1000);
            const questEnd = questTimerStart + questTimerDuration;
            const remaining = questEnd - now;

            console.log(`[QuestTimer] now: ${now} (${formatTime(now)}), end: ${questEnd} (${formatTime(questEnd)}), remaining: ${remaining}s`);
            if (questTimerDuration > 0 && remaining > 0) {
                if (submitBtn) {
                    submitBtn.disabled = true;
                    submitBtn.textContent = `⏳ Submit in ${remaining}s`;
                }
            } else {
                if (submitBtn) {
                    submitBtn.disabled = false;
                    submitBtn.textContent = "Submit";
                }
                clearInterval(questInterval);
            }
        }
        let questInterval;
        if (questTimerDuration > 0) {
            updateQuestTimer();
            questInterval = setInterval(updateQuestTimer, 1000);
        } else {
            if (submitBtn) {
                submitBtn.disabled = false;
                submitBtn.textContent = "Submit";
            }
        }

        // --- HINT TIMER ---
        function updateHintTimer() {
            const now = Math.floor(Date.now() / 1000);
            const hintEnd = hintTimerStart + hintTimerDuration;
            const remaining = hintEnd - now;

            console.log(`[HintTimer] now: ${now} (${formatTime(now)}), end: ${hintEnd} (${formatTime(hintEnd)}), remaining: ${remaining}s`);
            if (hintTimerDuration > 0 && remaining > 0) {
                if (hintBtn) {
                    hintBtn.disabled = true;
                    hintBtn.textContent = `🔒 Hint in ${remaining}s`;
                }
            } else {
                if (hintBtn) {
                    hintBtn.disabled = false;
                    hintBtn.textContent = "Show Hint";
                }
                clearInterval(hintInterval);
            }
        }
        let hintInterval;
        if (hintTimerDuration > 0 && hintBtn) {
            updateHintTimer();
            hintInterval = setInterval(updateHintTimer, 1000);
        } else if (hintBtn) {
            hintBtn.disabled = false;
            hintBtn.textContent = "Show Hint";
        }

        // Show/hide hint content
        if (hintBtn && hintContent) {
            hintBtn.addEventListener('click', function() {
                hintContent.style.display = hintContent.style.display === "none" ? "block" : "none";
                console.log("Hint button clicked, hintContent.style.display:", hintContent.style.display);
            });
        }
      })
      .catch(err => {
        console.error("Failed to load timers:", err);
      });
});
