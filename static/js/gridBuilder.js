// static/js/gridBuilder.js
// Collect grid digits row-by-row into #grid_cipher before form submit.
// Also save them as digits only and auto-advance focus.

window.addEventListener('DOMContentLoaded', function () {
  const form = document.getElementById('answer-form');
  const gridContainer = document.getElementById('grid-container');
  const gridCipherInput = document.getElementById('grid_cipher');

  if (!form || !gridContainer || !gridCipherInput) return;

  const cells = Array.from(gridContainer.querySelectorAll('.grid-cell'));

  // Keep only 0-9 and auto-advance
  cells.forEach((cell, idx) => {
    cell.addEventListener('input', () => {
      cell.value = cell.value.replace(/[^0-9]/g, '');
      if (cell.value.length === 1) {
        // move focus to next cell
        const next = cells[idx + 1];
        if (next) next.focus();
      }
    });

    cell.addEventListener('keydown', (e) => {
      if (e.key === 'Backspace' && !cell.value && idx > 0) {
        const prev = cells[idx - 1];
        if (prev) prev.focus();
      }
    });
  });

  form.addEventListener('submit', function () {
    // Build row-major string
    const digits = cells.map(c => (c.value || '').replace(/[^0-9]/g, '')).join('');
    gridCipherInput.value = digits;
  });
});
