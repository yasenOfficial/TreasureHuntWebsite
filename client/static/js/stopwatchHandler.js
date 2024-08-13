const startTime = new Date(document.getElementById("start-time").getAttribute("data-start-time"));
const elapsedTimeElement = document.getElementById("elapsed-time");

function updateElapsedTime() {
    const now = new Date();
    const elapsed = new Date(now - startTime);

    const hours = String(elapsed.getUTCHours()).padStart(2, '0');
    const minutes = String(elapsed.getUTCMinutes()).padStart(2, '0');
    const seconds = String(elapsed.getUTCSeconds()).padStart(2, '0');

    elapsedTimeElement.textContent = `${hours}:${minutes}:${seconds}`;
}

// Update the timer every second
setInterval(updateElapsedTime, 1000);

// Initial update
updateElapsedTime();
