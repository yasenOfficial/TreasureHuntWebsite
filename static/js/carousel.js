// static/js/carousel.js
document.addEventListener('DOMContentLoaded', function () {
  // Helper: get active slide index (0-based)
  function getActiveIndex(carouselEl) {
    const items = Array.from(carouselEl.querySelectorAll('.carousel-item'));
    const idx = items.findIndex(i => i.classList.contains('active'));
    return Math.max(0, idx);
  }

  function hideControlsAndCounter(containerEl, counterEl) {
    // Hide counter
    if (counterEl) counterEl.style.display = 'none';
    // Hide prev/next buttons (within this container)
    containerEl.querySelectorAll('.carousel-control-prev, .carousel-control-next')
      .forEach(btn => (btn.style.display = 'none'));
  }

  function initCarousel({ carouselSel, containerSel, counterSel }) {
    const carouselEl = document.querySelector(carouselSel);
    const containerEl = document.querySelector(containerSel);
    let counterEl = counterSel ? document.querySelector(counterSel) : null;

    if (!carouselEl || !containerEl) return;

    // If no explicit counter selector, try the data-based one inside container
    if (!counterEl) {
      counterEl = containerEl.querySelector('[data-carousel="counter"]');
    }

    // Determine total images: prefer data attribute, else count items
    const totalImages =
      parseInt(containerEl.dataset.totalImages || '0', 10) ||
      carouselEl.querySelectorAll('.carousel-item').length;

    // If 0 or 1 images → hide counter & controls and skip listeners
    if (!totalImages || totalImages === 1) {
      hideControlsAndCounter(containerEl, counterEl);
      return;
    }

    // Ensure Bootstrap Carousel exists
    if (typeof bootstrap === 'undefined' || !bootstrap.Carousel) return;
    bootstrap.Carousel.getOrCreateInstance(carouselEl);

    function updateCounter(currentIndex) {
      if (!counterEl) return;
      counterEl.textContent = `${currentIndex + 1}/${totalImages}`;
      counterEl.style.display = ''; // ensure visible if multiple images
    }

    // Initialize counter with actual active slide
    updateCounter(getActiveIndex(carouselEl));

    // Update after each slide completes
    carouselEl.addEventListener('slid.bs.carousel', function (event) {
      if (typeof event.to === 'number') {
        updateCounter(event.to);
      } else {
        updateCounter(getActiveIndex(carouselEl));
      }
    });
  }

  // Generic initializer for any data-attribute carousels
  function initAllDataCarousels() {
    document.querySelectorAll('[data-carousel="container"]').forEach(container => {
      const targetId = container.getAttribute('data-target');
      if (!targetId) return;

      initCarousel({
        carouselSel: `#${targetId}`,
        containerSel: `[data-carousel="container"][data-target="${targetId}"]`,
        counterSel: null, // counter is found inside container via [data-carousel="counter"]
      });
    });
  }

  // 1) Data-attribute carousels (covers your hint carousel from the template)
  initAllDataCarousels();

  // 2) Legacy ID-based quest carousel (your existing markup)
  initCarousel({
    carouselSel: '#questImagesCarousel',
    containerSel: '#carousel-container',
    counterSel: '#carousel-image-counter',
  });

  // 3) Optional: ID-based hint carousel (if you choose to wire it by IDs)
  initCarousel({
    carouselSel: '#hintImagesCarousel',
    containerSel: '#hint-carousel-container',
    counterSel: '#hint-carousel-image-counter',
  });
});
