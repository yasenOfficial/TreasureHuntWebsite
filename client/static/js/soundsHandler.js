// Play the success sound when the success modal is shown
$('#successModal').on('shown.bs.modal', function () {
    var audio = new Audio('/static/audio/duolingo-right.mp3');
    audio.play();
});

// Play the skip sound when the skip modal is shown
$('#skipModal').on('shown.bs.modal', function () {
    var audio = new Audio('/static/audio/duolingo-wrong.mp3');
    audio.play();
});
