const remoteCtrl = document.getElementById('remote');
const apiRoot = "/api/v1"
const call = apiRoot + "/call"
const pwrStatus = apiRoot + "/pwr";

const pwrButton = document.getElementById("power");
const channel = document.getElementById("channel")

// Check power status
const interval = setInterval(function () {
    checkStatus()
}, 10000)

function checkStatus() {
    const xmlHttp = new XMLHttpRequest();
    xmlHttp.open("GET", pwrStatus, false); // false for synchronous request
    xmlHttp.send(null);
    const resp = JSON.parse(xmlHttp.responseText)
    const pwrClass = pwrButton.classList;
    if (resp.off === false) {
        toggleEnabled(true);
        pwrClass.add("btn-danger")
        pwrClass.remove("btn-success")
    } else {
        toggleEnabled(false);
        pwrClass.add("btn-success")
        pwrClass.remove("btn-danger")
    }
}

function sendCmd(cmd) {
    const xmlHttp = new XMLHttpRequest();
    xmlHttp.open("GET", call + "/" + cmd, false); // false for synchronous request
    xmlHttp.send(null);
    checkStatus();
}

remoteCtrl.addEventListener('click', (event) => {
    let id = "";
    if (event.target.nodeName === 'BUTTON') {
        id = event.target.id;
    } else if (event.target.nodeName === 'I') {
        // If it's one of the icons, process its parent
        id = event.target.parentElement.id;
    } else {
        return
    }
    if (id === "ch") {
        channelInput();
    } else {
        // Send the raw request
        sendCmd(id);
    }
})

channel.addEventListener('keypress', (event) => {
    if (event.key === "Enter") {
        // Split channel input into separate values
        channelInput()
    }
})

function channelInput() {
    const nums = channel.value.toString().split('');

    for (let i = 0; i < nums.length; i++) {
        sendCmd(nums[i])
        sleep(500);
    }
}

// Keep the player variable in a scope accessible by your functions
let player;
const videoElement = document.getElementById('slinky-video');
const sourceDropdown = document.getElementById("quality");

// 1. Create a function to change the stream
function changeStream(url) {
    // If a player instance already exists, destroy it
    if (player) {
        player.destroy();
    }

    // Create a new player with the new URL
    player = mpegts.createPlayer({
        type: 'mpegts',
        isLive: true,
        url: url // Use the URL passed to the function
    });

    // Attach, load, and play
    player.attachMediaElement(videoElement);
    player.load();
    player.play();
}

function initVideo() {
    if (mpegts.isSupported()) {
        const source = document.getElementById("quality");
        // Add the event listener to the dropdown
        source.addEventListener('change', (event) => {
            changeStream(event.target.value);
        });

        // 3. Load the initial stream using the dropdown's current value
        changeStream(sourceDropdown.value);
    } else {
        console.error("mpegts.js is not supported in this browser.");
    }
}

function toggleEnabled(enabled) {
    const elements = remoteCtrl.getElementsByTagName('button')
    for (let i = 0; i < elements.length; i++) {
        if (elements[i].id !== "power") {
            //elements[i].removeAttribute("style");
            elements[i].disabled = !enabled;
        }
    }
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

window.HELP_IMPROVE_VIDEOJS = false;