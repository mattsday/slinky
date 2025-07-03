document.addEventListener('DOMContentLoaded', () => {
    // Set up all event listeners and initial states
    initRemoteControls();
    initVideoPlayer();
    initCasting();

    // Perform initial status check
    checkStatus();
    // Set up the recurring status check
    setInterval(checkStatus, 10000);
});

// --- Constants ---
const apiRoot = "/api/v1";
const pwrButton = document.getElementById("power");

// --- Core Functions ---

async function checkStatus() {
    try {
        const response = await fetch(`${apiRoot}/pwr`);
        if (!response.ok) {
            throw new Error(`Server responded with ${response.status}`);
        }
        const resp = await response.json();
        const isOff = resp.off !== false; // Treat any non-false value as off

        toggleRemoteEnabled(!isOff);
        pwrButton.classList.toggle("btn-success", isOff);
        pwrButton.classList.toggle("btn-danger", !isOff);
        pwrButton.ariaPressed = !isOff ? "true" : "false";

    } catch (error) {
        console.error("Failed to check power status:", error);
        toggleRemoteEnabled(false); // Disable remote if server is unreachable
        pwrButton.classList.add("btn-success");
        pwrButton.classList.remove("btn-danger");
    }
}

async function sendCmd(cmd) {
    try {
        await fetch(`${apiRoot}/call/${cmd}`);
        await checkStatus(); // Update status immediately after a command
    } catch (error) {
        console.error(`Failed to send command ${cmd}:`, error);
    }
}

async function channelInput() {
    const channel = document.getElementById("channel");
    const nums = channel.value.toString().split('');
    channel.disabled = true; // Prevent input during command sequence

    for (const num of nums) {
        await sendCmd(num);
        await sleep(400); // Wait between sending each digit
    }
    channel.value = ""; // Clear input after sending
    channel.disabled = false;
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

// --- Initialization ---

function initRemoteControls() {
    const remoteCtrl = document.getElementById('remote');
    const channel = document.getElementById("channel");

    remoteCtrl.addEventListener('click', (event) => {
        const button = event.target.closest('button');
        if (!button || button.disabled) return;

        if (button.id === "ch") {
            channelInput();
        } else {
            sendCmd(button.id);
        }
    });

    channel.addEventListener('keypress', (event) => {
        if (event.key === "Enter") {
            channelInput();
        }
    });
}

function initVideoPlayer() {
    if (!mpegts.isSupported()) {
        console.error("mpegts.js is not supported in this browser.");
        return;
    }

    const videoElement = document.getElementById('slinky-video');
    const sourceDropdown = document.getElementById("quality");
    let player;

    const changeStream = (url) => {
        if (player) {
            player.destroy();
        }
        player = mpegts.createPlayer({
            type: 'mpegts',
            isLive: true,
            url: url
        });
        player.attachMediaElement(videoElement);
        player.load();
        player.play();
    };

    sourceDropdown.addEventListener('change', (event) => {
        changeStream(event.target.value);
    });

    changeStream(sourceDropdown.value); // Load initial stream
}

function initCasting() {
    const video = document.getElementById('slinky-video');
    const castButton = document.getElementById('castButton');
    if (!castButton) return;

    if ('remote' in video) {
        video.remote.watchAvailability((isAvailable) => {
            castButton.style.display = isAvailable ? 'block' : 'none';
        }).catch(() => {
            castButton.style.display = 'none';
        });

        castButton.addEventListener('click', () => {
            video.remote.prompt().catch((error) => {
                console.error('Error opening remote playback prompt:', error);
            });
        });
    }
}

function toggleRemoteEnabled(enabled) {
    const remoteCtrl = document.getElementById('remote');
    const remoteButtons = remoteCtrl.getElementsByTagName('button');
    for (const button of remoteButtons) {
        if (button.id !== "power") {
            button.disabled = !enabled;
        }
    }
    // Also disable/enable the channel input field
    document.getElementById("channel").disabled = !enabled;
}
