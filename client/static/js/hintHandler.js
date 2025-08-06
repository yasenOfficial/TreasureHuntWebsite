document.addEventListener("DOMContentLoaded", function () {
    var hintButton = document.getElementById("hintButton");
    var hintText = document.getElementById("hintText");
    var hintCount = document.getElementById("hintCount");
    var questId = hintButton ? hintButton.getAttribute("data-quest-id") : null;
    // console.log("questId: " + questId);


    // Check if the hint has already been shown by looking in localStorage
    if (localStorage.getItem("hintShown_" + questId) === "true") {
        if (hintText) {
            hintText.style.display = "block";
        }
        if (hintButton) {
            hintButton.disabled = true;
        }
    }

    if (hintButton) {
        hintButton.addEventListener("click", function () {
            if (hintText) {
                hintText.style.display = "block";

                // Disable the hint button
                hintButton.disabled = true;

                // Store the hint status in localStorage
                localStorage.setItem("hintShown_" + questId, "true");

                // Send a request to increment the hint count
                fetch(`/hint/${questId}`, { method: 'POST' })
                    .then(response => {
                        if (!response.ok) {
                            throw new Error('Network response was not ok');
                        }
                        return response.json();
                    })
                    .then(data => {
                        if (data.success) {
                            console.log("Hint count incremented.");
                            // Update the hint count display
                            if (hintCount) {
                                var currentHintCount = parseInt(hintCount.textContent.split(": ")[1]);
                                hintCount.textContent = "Hints used: " + (currentHintCount + 1);
                            }
                        } else {
                            console.error("Failed to increment hint count.");
                        }
                    })
                    .catch(error => {
                        console.error("There was a problem with the fetch operation:", error);
                    });
            }
        });
    }

    const DEBUG = false; // Set to false to disable debugging logs

    const timerEndTimeElement = document.getElementById("hint-timer-end-time");
    const timerRemainingElement = document.getElementById("hint-timer");

    if (timerEndTimeElement && timerRemainingElement) {
        // Debug: Check if the element and data are found
        if (DEBUG) {
            console.log('Timer end time element found:', timerEndTimeElement);
            console.log('Timer remaining element found:', timerRemainingElement);
        }

        const timerEndTimeStr = timerEndTimeElement.getAttribute("data-end-time");
        if (DEBUG) {
            console.log('Data end time attribute:', timerEndTimeStr);
        }

        const timerEndTime = new Date(timerEndTimeStr);
        if (DEBUG) {
            console.log('Parsed end time:', timerEndTime);
        }

        function updateTimer() {
            const now = new Date();
            const remainingTime = timerEndTime - now;

            // Debug: Log current time and remaining time
            if (DEBUG) {
                console.log('Current time:', now);
                console.log('Remaining time (ms):', remainingTime);
            }

            if (remainingTime <= 0) {
                // timerRemainingElement.textContent = "Time's up!";
                clearInterval(timerInterval);
                if (DEBUG) {
                    console.log("Timer expired");
                }

                // Fetch and reload the current URL
                // const currentUrl = window.location.href;
                hintButton.disabled = false;
                timerRemainingElement.textContent = "";

                // window.location.reload(); // Reload the page with the current URL

                return;
            }else{
                hintButton.disabled = true;
            }

            const hours = String(Math.floor(remainingTime / (1000 * 60 * 60))).padStart(2, '0');
            const minutes = String(Math.floor((remainingTime % (1000 * 60 * 60)) / (1000 * 60))).padStart(2, '0');
            const seconds = String(Math.floor((remainingTime % (1000 * 60)) / 1000)).padStart(2, '0');

            // Debug: Log formatted time
            if (DEBUG) {
                console.log('Formatted time:', `${hours}:${minutes}:${seconds}`);
            }

            timerRemainingElement.textContent = `Hint Timer: ${minutes}:${seconds}`;

        }

        // Update the timer every second
        const timerInterval = setInterval(updateTimer, 1000);

        // Initial update
        updateTimer();
    } else {
        if (DEBUG) {
            console.log('Timer elements not found');
        }
    }
});