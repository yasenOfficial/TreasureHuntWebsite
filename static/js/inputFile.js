window.addEventListener('DOMContentLoaded', function () {
  const fileInputs = document.querySelectorAll('input[type="file"]');
  
  fileInputs.forEach(input => {
    // Create a wrapper for better styling - NO BORDER
    const wrapper = document.createElement('div');
    wrapper.className = 'file-input-wrapper';
    wrapper.style.cssText = `
      position: relative;
      display: flex;
      align-items: center;
      width: 100%;
      height: 3.5rem;
      margin-bottom: 1.5rem;
      border: 2px solid var(--accent-copper);
      border-radius: 13px;
      background: var(--bg-primary);
      transition: all 0.3s ease;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
      overflow: hidden;
    `;

    // Create the button part
    const button = document.createElement('button');
    button.type = 'button';
    button.textContent = 'Изберете файл';
    button.style.cssText = `
      height: 100%;
      border: none;
      background: linear-gradient(145deg, var(--accent-gold), var(--accent-copper));
      color: var(--text-dark);
      font-family: 'Cinzel', serif;
      font-size: clamp(0.9rem, 2vw, 1rem);
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 1px;
      cursor: pointer;
      transition: all 0.3s ease;
      padding: 0 1rem;
      border-radius: 0;
      border-top-left-radius: 6px;
      border-bottom-left-radius: 6px;
      flex-shrink: 0;
      min-width: 120px;
    `;

    // Create the filename display
    const filename = document.createElement('span');
    filename.textContent = 'Не е избран файл';
    filename.style.cssText = `
      flex: 1;
      padding: 0 1rem;
      color: var(--accent-copper);
      font-family: 'Cinzel', serif;
      font-size: clamp(0.9rem, 2vw, 1rem);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      display: flex;
      align-items: center;
      height: 100%;
    `;

    // Hide the original input
    input.style.cssText = `
      position: absolute;
      opacity: 0;
      width: 100%;
      height: 100%;
      cursor: pointer;
      z-index: 2;
    `;

    // Insert wrapper before input
    input.parentNode.insertBefore(wrapper, input);
    
    // Add elements to wrapper
    wrapper.appendChild(button);
    wrapper.appendChild(filename);
    wrapper.appendChild(input);

    // Handle file selection
    input.addEventListener('change', function() {
      if (this.files && this.files.length > 0) {
        const file = this.files[0];
        filename.textContent = file.name;
        filename.style.color = 'var(--accent-gold)';
      } else {
        filename.textContent = 'No file chosen';
        filename.style.color = 'var(--accent-copper)';
      }
    });

    // Handle hover effects - NO BORDER CHANGES
    wrapper.addEventListener('mouseenter', function() {
      this.style.transform = 'translateY(-2px)';
      this.style.boxShadow = '0 6px 20px rgba(212, 175, 55, 0.4)';
      button.style.background = 'linear-gradient(145deg, var(--accent-copper), var(--accent-gold))';
    });

    wrapper.addEventListener('mouseleave', function() {
      this.style.transform = 'translateY(0)';
      this.style.boxShadow = '0 2px 8px rgba(0, 0, 0, 0.3)';
      button.style.background = 'linear-gradient(145deg, var(--accent-gold), var(--accent-copper))';
    });

    // Handle button click
    button.addEventListener('click', function() {
      input.click();
    });
  });
});