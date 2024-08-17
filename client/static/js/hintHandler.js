document.addEventListener("DOMContentLoaded", function () {
    var hintButton = document.getElementById("hintButton");
    var hintText = document.getElementById("hintText");
    var hintCount = document.getElementById("hintCount");
    var questId = hintButton ? hintButton.getAttribute("data-quest-id") : null;
    console.log("questId: " + questId);


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
});