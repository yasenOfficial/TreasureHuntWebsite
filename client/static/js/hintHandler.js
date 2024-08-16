document.addEventListener("DOMContentLoaded", function() {
    var hintButton = document.getElementById("hintButton");

    if (hintButton) {
        hintButton.addEventListener("click", function () {
            var hintText = document.getElementById("hintText");
            var hintCount = document.getElementById("hintCount");

            if (hintText) {
                hintText.style.display = "block";

                // Disable the hint button
                hintButton.disabled = true;

                // Send a request to increment the hint count
                fetch(`/hint/${this.getAttribute("data-quest-id")}`, { method: 'POST' })
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
