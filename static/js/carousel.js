document.addEventListener('DOMContentLoaded', function () {
  const carousel = document.querySelector('#questImagesCarousel');
  const counter = document.querySelector('#carousel-image-counter');

  if (!carousel || !counter) return;

  // Total images from data attribute
  const totalImages = parseInt(
    document.querySelector('#carousel-container').dataset.totalImages
  );

  // Function to update the counter
  function updateCounter(index) {
    counter.textContent = `${index + 1}/${totalImages}`;
  }

  // Initialize counter to first slide
  updateCounter(0);

  // Listen to Bootstrap's carousel 'slid' event (after slide completes)
  carousel.addEventListener('slid.bs.carousel', function (event) {
    updateCounter(event.to); // event.to is zero-based index of current slide
  });
});
