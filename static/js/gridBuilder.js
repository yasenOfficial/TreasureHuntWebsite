
    (function() {
      const form = document.getElementById('answer-form');
      if (!form) return;

      const gridContainer = document.getElementById('grid-container');
      const hiddenCipher = document.getElementById('grid_cipher');

      // Only apply if there's a grid on the page
      if (gridContainer && hiddenCipher) {
        // Force uppercase letters/digits only (optional UI nicety)
        gridContainer.addEventListener('input', function(e) {
          const el = e.target;
          if (el.classList.contains('grid-cell')) {
            el.value = el.value.replace(/[^a-zA-Z0-9]/g, '').toLowerCase().slice(0, 1);
          }
        });

        form.addEventListener('submit', function() {
          const cells = Array.from(gridContainer.querySelectorAll('.grid-cell'));
          // Sort by row-major
          cells.sort((a, b) => {
            const ar = parseInt(a.dataset.r, 10), ac = parseInt(a.dataset.c, 10);
            const br = parseInt(b.dataset.r, 10), bc = parseInt(b.dataset.c, 10);
            if (ar === br) return ac - bc;
            return ar - br;
          });

          // Join into one string
          const cipher = cells.map(el => (el.value || '')).join('').trim().toLowerCase();
          hiddenCipher.value = cipher;
        });
      }

      // Hint toggle
      // Hint toggle
      const hintBtn = document.getElementById('hint-btn');
      const hintContent = document.getElementById('hint-content');

      if (hintBtn && hintContent) {
        hintBtn.addEventListener('click', () => {
          const isHidden = hintContent.style.display === 'none' || hintContent.style.display === '';
          hintContent.style.display = isHidden ? 'block' : 'none';
          hintBtn.textContent = isHidden ? 'Hide Hint' : 'Show Hint';
        });
      }
    })();
