function checkGameFinished() {
    fetch('/treasurehunt?team=' + getTeamName(), { credentials: 'include' })
        .then(response => {
            if (response.redirected) {
                window.location.href = response.url;
            }
        });
}

function getTeamName() {
    // Function to get the team name from cookies or other means
    return document.cookie.replace(/(?:(?:^|.*;\s*)logged_in_team\s*\=\s*([^;]*).*$)|^.*$/, "$1");
}

setInterval(checkGameFinished, 5000); // Check every 5 secs