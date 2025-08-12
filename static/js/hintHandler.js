document.addEventListener("DOMContentLoaded", () => {
  const hintBtn = document.getElementById("hint-btn");
  const hintContent = document.getElementById("hint-content");

  if (hintBtn && hintContent) {
    hintBtn.addEventListener("click", () => {
      fetch("/api/use_hint", {
        method: "POST",
        headers: {
          "X-Requested-With": "XMLHttpRequest"
        }
      })
        .then(res => {
          if (!res.ok) throw new Error("Failed to mark hint usage");
          return res.json();
        })
        .then(data => {
          console.log("Hint usage recorded:", data);
          hintContent.style.display = "block";
          hintBtn.style.display = "none"; // Hide button after showing
        })
        .catch(err => {
          console.error(err);
          alert("Error showing hint. Please try again.");
        });
    });
  }
});
