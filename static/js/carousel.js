// carousel.js

document.addEventListener('DOMContentLoaded', function () {
  const carousel = document.querySelector('#questImagesCarousel');
  const counter = document.querySelector('#carousel-image-counter');

  // Get the total number of images from the data attribute in HTML
  const totalImages = document.querySelector('#carousel-container').getAttribute('data-total-images');
  let currentIndex = 0; // Tracks the current image index

  // Function to update the counter
  function updateCounter() {
    counter.textContent = `${currentIndex + 1}/${totalImages}`;
  }

  // Initialize counter
  updateCounter();

  // Update the image counter when the carousel slides
  carousel.addEventListener('slide.bs.carousel', function (event) {
    currentIndex = event.to;  // event.to is the index of the next slide
    updateCounter();
  });

  // Event listeners for the carousel controls (prev/next)
  const prevButton = document.querySelector('.carousel-control-prev');
  const nextButton = document.querySelector('.carousel-control-next');

  prevButton.addEventListener('click', function () {
    if (currentIndex > 0) {
      currentIndex--;
    } else {
      currentIndex = totalImages - 1;  // Wrap around to the last image
    }
    updateCounter();
  });

  nextButton.addEventListener('click', function () {
    if (currentIndex < totalImages - 1) {
      currentIndex++;
    } else {
      currentIndex = 0;  // Wrap around to the first image
    }
    updateCounter();
  });
});
