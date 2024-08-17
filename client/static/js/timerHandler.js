document.addEventListener("DOMContentLoaded", function() {
    console.log("JavaScript file loaded and DOM content is ready.");

    const startTimeElement = document.getElementById("start-time");
    const countdownElement = document.getElementById("countdown-time");

    if (startTimeElement && countdownElement) {
        const startTime = new Date(startTimeElement.getAttribute("data-start-time"));
        const twoHours = 2 * 60 * 60 * 1000; // 2 hours in milliseconds
        const endTime = new Date(startTime.getTime() + twoHours);

        function updateCountdown() {
            const now = new Date();
            const remainingTime = endTime - now;

            if (remainingTime <= 0) {
                countdownElement.textContent = "00:00:00";
                clearInterval(timerInterval);
                return;
            }

            const hours = String(Math.floor(remainingTime / (1000 * 60 * 60))).padStart(2, '0');
            const minutes = String(Math.floor((remainingTime % (1000 * 60 * 60)) / (1000 * 60))).padStart(2, '0');
            const seconds = String(Math.floor((remainingTime % (1000 * 60)) / 1000)).padStart(2, '0');

            countdownElement.textContent = `${hours}:${minutes}:${seconds}`;
        }

        const timerInterval = setInterval(updateCountdown, 1000);
        updateCountdown();
    }
});
