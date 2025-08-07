window.addEventListener('DOMContentLoaded', function () {
    console.log("Fetching /api/timers...");

    function formatTimeLeft(seconds) {
        if (seconds < 0) seconds = 0;
        const hrs = Math.floor(seconds / 3600);
        const mins = Math.floor((seconds % 3600) / 60);
        const secs = seconds % 60;
        return `${hrs.toString().padStart(2, '0')}:${mins
            .toString()
            .padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    }

    fetch('/api/timers')
        .then(res => res.json())
        .then(data => {
            console.log("Timer data:", data);

            const questTimerDuration = data.questTimerDuration;
            const questTimerStart = data.questTimerStart;
            const hintTimerDuration = data.hintTimerDuration;
            const hintTimerStart = data.hintTimerStart;
            const overallStart = data.overallTimerStart || questTimerStart; // fallback if not sent
            const overallDuration = 7200; // 2 hours in seconds

            // Get DOM elements
            const submitBtn = document.getElementById('submit-btn');
            const hintBtn = document.getElementById('hint-btn');
            const hintContent = document.getElementById('hint-content');
            const globalTimerElem = document.getElementById('global-timer');

            // --- Global/Overall 2-hour timer ---
            function updateGlobalTimer() {
                if (!globalTimerElem) return; // Safety check
                
                const now = Math.floor(Date.now() / 1000);
                const end = overallStart + overallDuration;
                const remaining = end - now;

                globalTimerElem.textContent = `⏰ Total Time Remaining: ${formatTimeLeft(remaining)}`;

                if (remaining <= 0) {
                    globalTimerElem.textContent = "⏰ Time's up!";
                    globalTimerElem.style.color = 'var(--error)';
                    clearInterval(globalInterval);
                    // Lock submit when overall timer runs out
                    if (submitBtn) submitBtn.disabled = true;
                }
            }
            
            if (globalTimerElem) {
                updateGlobalTimer();
                var globalInterval = setInterval(updateGlobalTimer, 1000);
            }

            // --- Quest delay timer (updates submit button only) ---
            function updateQuestTimer() {
                const now = Math.floor(Date.now() / 1000);
                const questEnd = questTimerStart + questTimerDuration;
                const remaining = questEnd - now;

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
            
            if (questTimerDuration > 0) {
                updateQuestTimer();
                var questInterval = setInterval(updateQuestTimer, 1000);
            }

            // --- Hint delay timer (updates hint button only) ---
            function updateHintTimer() {
                const now = Math.floor(Date.now() / 1000);
                const hintEnd = hintTimerStart + hintTimerDuration;
                const remaining = hintEnd - now;

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
            
            if (hintTimerDuration > 0 && hintBtn) {
                updateHintTimer();
                var hintInterval = setInterval(updateHintTimer, 1000);
            }

            // --- Hint toggle ---
            if (hintBtn && hintContent) {
                hintBtn.addEventListener('click', function () {
                    if (hintContent.style.display === "none" || hintContent.style.display === "") {
                        hintContent.style.display = "block";
                        if (!hintBtn.disabled) {
                            hintBtn.textContent = "Hide Hint";
                        }
                    } else {
                        hintContent.style.display = "none";
                        if (!hintBtn.disabled) {
                            hintBtn.textContent = "Show Hint";
                        }
                    }
                });
            }

            // --- Skip button functionality ---
            const skipBtn = document.getElementById('skip-btn');
            if (skipBtn) {
                skipBtn.addEventListener('click', function(e) {
                    e.preventDefault(); // Prevent form submission
                    
                    // Create a form to submit the skip action
                    const skipForm = document.createElement('form');
                    skipForm.method = 'POST';
                    skipForm.action = '/skip';
                    skipForm.style.display = 'none';
                    
                    document.body.appendChild(skipForm);
                    skipForm.submit();
                });
            }
        })
        .catch(err => {
            console.error("Failed to load timers:", err);
            
            // Fallback: Show error in global timer if it exists
            const globalTimerElem = document.getElementById('global-timer');
            if (globalTimerElem) {
                globalTimerElem.textContent = "❌ Timer loading failed";
                globalTimerElem.style.color = 'var(--error)';
            }
        });
});