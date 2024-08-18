const DEBUG = false; // Set to false to disable debugging logs

document.addEventListener("DOMContentLoaded", function() {
    const timerEndTimeElement = document.getElementById("quest-timer-end-time");
    const timerRemainingElement = document.getElementById("quest-timer");

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
                timerRemainingElement.textContent = "Time's up!";
                clearInterval(timerInterval);
                if (DEBUG) {
                    console.log("Timer expired");
                }

                // Fetch and reload the current URL
                // const currentUrl = window.location.href;
                window.location.reload(); // Reload the page with the current URL

                return;
            }

            const hours = String(Math.floor(remainingTime / (1000 * 60 * 60))).padStart(2, '0');
            const minutes = String(Math.floor((remainingTime % (1000 * 60 * 60)) / (1000 * 60))).padStart(2, '0');
            const seconds = String(Math.floor((remainingTime % (1000 * 60)) / 1000)).padStart(2, '0');

            // Debug: Log formatted time
            if (DEBUG) {
                console.log('Formatted time:', `${hours}:${minutes}:${seconds}`);
            }

            timerRemainingElement.textContent = `${hours}:${minutes}:${seconds}`;
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