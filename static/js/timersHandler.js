// static/js/timersHandler.js
window.addEventListener('DOMContentLoaded', function () {
  // Toasts: read ?status in URL and show
  (function showStatusToastIfAny() {
    const params = new URLSearchParams(window.location.search);
    const status = params.get("status");
    if (!status) return;

    const toastEl = document.getElementById("status-toast");
    const toastBody = document.getElementById("status-toast-message");
    if (!toastEl || !toastBody || typeof bootstrap === 'undefined' || !bootstrap.Toast) {
      // Clean URL even if toast lib isn't present
      window.history.replaceState({}, document.title, window.location.pathname);
      return;
    }

    if (status === "success") {
      toastEl.classList.remove("text-bg-danger");
            toastEl.classList.remove("text-bg-warning");

      toastEl.classList.add("text-bg-success");
      toastBody.textContent = "✅ Quest submitted successfully!";
    } else if (status === "skip") {
      toastEl.classList.remove("text-bg-success");
    toastEl.classList.remove("text-bg-danger");
      toastEl.classList.add("text-bg-warning");
      toastBody.textContent = "⏩ Quest Skipped"
    } else if (status === "wrong") {
      toastEl.classList.remove("text-bg-success");
    toastEl.classList.remove("text-bg-warning");
      toastEl.classList.add("text-bg-danger");
      toastBody.textContent = "❌ Wrong answer. Try again!";
    } else {
      toastBody.textContent = status;
    }

    const toast = new bootstrap.Toast(toastEl);
    toast.show();
    window.history.replaceState({}, document.title, window.location.pathname);
  })();

  // Format time helper
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
      if (data.error) {
        const gt = document.getElementById('global-timer');
        if (gt) {
          gt.textContent = "⛔ " + data.error;
        }
        return;
      }

      // From server (dynamic, no hardcode)
      const questTimerDuration = data.questTimerDuration;
      const questTimerStart = data.questTimerStart;
      const hintTimerDuration = data.hintTimerDuration;
      const hintTimerStart = data.hintTimerStart;
      const overallStart = data.globalTimerStart;
      const overallDuration = data.globalTimerDuration;

      const submitBtn = document.getElementById('submit-btn');
      const hintBtn = document.getElementById('hint-btn');
      const hintContent = document.getElementById('hint-content');
      const globalTimerElem = document.getElementById('global-timer');

      // --- Global Timer ---
      function updateGlobalTimer() {
        if (!globalTimerElem) return;
        const now = Math.floor(Date.now() / 1000);
        const end = overallStart + overallDuration;
        const remaining = end - now;

        globalTimerElem.textContent = `⏰ Time Remaining: ${formatTimeLeft(remaining)}`;

        if (remaining <= 0) {
          globalTimerElem.textContent = "⏰ Time's up!";
          if (submitBtn) submitBtn.disabled = true;
          clearInterval(globalInterval);
        }
      }
      updateGlobalTimer();
      const globalInterval = setInterval(updateGlobalTimer, 1000);

      // --- Quest Timer (submit disabled while waiting) ---
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

      // --- Hint Timer (toggle button) ---
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
            // keep current label if content is visible
            if (hintContent && hintContent.style.display === "block") {
              hintBtn.textContent = "Hide Hint";
            } else {
              hintBtn.textContent = "Show Hint";
            }
          }
          clearInterval(hintInterval);
        }
      }
      if (hintBtn && hintTimerDuration > 0) {
        updateHintTimer();
        var hintInterval = setInterval(updateHintTimer, 1000);
      } else if (hintBtn) {
        hintBtn.disabled = false;
      }

      // Hint click toggle
      if (hintBtn && hintContent) {
        hintBtn.addEventListener('click', function () {
          const visible = (hintContent.style.display === "block");
          hintContent.style.display = visible ? "none" : "block";
          if (!hintBtn.disabled) {
            hintBtn.textContent = visible ? "Show Hint" : "Hide Hint";
          }
        });
      }
    })
    .catch(err => {
      console.error("Failed to load timers:", err);
      const globalTimerElem = document.getElementById('global-timer');
      if (globalTimerElem) {
        globalTimerElem.textContent = "❌ Timer loading failed";
      }
    });
});
