// checkQuestStatus.js

async function checkQuestStatus() {
    try {
        const response = await fetch('/check-quest-status');
        if (!response.ok) {
            throw new Error('Network response was not ok');
        }
        const status = await response.json();

        // Extract the current quest number from the server response
        const currentQuestNumber = status.questNumber;

        // Get the current quest number from the webpage
        const displayedQuestNumber = parseInt(document.getElementById('current-quest').textContent, 10);

        // If the quest number has changed, refresh the page
        if (currentQuestNumber !== displayedQuestNumber) {
            window.location.reload();
        }
    } catch (error) {
        console.error('Error checking quest status:', error);
    }
}

// Check the quest status every 5 seconds
setInterval(checkQuestStatus, 5000);
