document.addEventListener('DOMContentLoaded', function() {
    // Check if there's a success message in the URL parameters
    const urlParams = new URLSearchParams(window.location.search);
    const successMsg = urlParams.get('success');
    
    // Show success message if present
    if (successMsg) {
        const successAlert = document.createElement('div');
        successAlert.className = 'alert alert-success mt-4';
        successAlert.style.backgroundColor = 'lightgreen';
        successAlert.style.color = '#333';
        successAlert.innerHTML = `<strong>Congratulations! </strong> ${successMsg}`;
        document.body.insertBefore(successAlert, document.querySelector('.container'));

        // Automatically hide the success message after 5 seconds
        setTimeout(() => {
            successAlert.style.opacity = '0';
            setTimeout(() => {
                successAlert.remove();
            }, 500); // Delay to allow for fade-out effect
        }, 5000); // Show for 5 seconds
    }

    // Check if there's an error message to display
    const errorAlert = document.getElementById('error-alert');
    if (errorAlert) {
        // Automatically hide the error message after 5 seconds
        setTimeout(() => {
            errorAlert.style.opacity = '0';
            setTimeout(() => {
                errorAlert.remove();
            }, 500); // Delay to allow for fade-out effect
        }, 5000); // Show for 5 seconds
    }
});
