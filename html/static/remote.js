import { sendCmd, checkStatus } from './api.js';

const pwrButton = document.getElementById("power");
const remoteCtrl = document.getElementById('remote');
const channel = document.getElementById("channel");
const videoElement = document.getElementById('slinky-video'); // Get the video player element
const muteButton = document.getElementById('mute'); // Get the mute button

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function channelInput() {
  const nums = channel.value.toString().split('');
  channel.disabled = true;

  for (const num of nums) {
    await sendCmd(num);
    await sleep(400);
  }
  channel.value = "";
  channel.disabled = false;
}

function toggleRemoteEnabled(enabled) {
  const remoteButtons = remoteCtrl.getElementsByTagName('button');
  for (const button of remoteButtons) {
    if (button.id !== "power") {
      button.disabled = !enabled;
    }
  }
  channel.disabled = !enabled;
}

// Function to update the mute button's visual state
function updateMuteButtonStatus() {
  if (videoElement && muteButton) {
    muteButton.classList.toggle('btn-danger', videoElement.muted);
  }
}

export function initRemoteControls() {
  if (!remoteCtrl) return;

  // Listen for volume changes on the video to update the mute button
  if (videoElement) {
    videoElement.addEventListener('volumechange', updateMuteButtonStatus);
  }

  remoteCtrl.addEventListener('click', async (event) => {
    const button = event.target.closest('button');
    if (!button || button.disabled) return;

    const buttonId = button.id;

    // Add visual feedback to the clicked button
    button.classList.add('btn-clicked');
    setTimeout(() => {
      button.classList.remove('btn-clicked');
    }, 200); // Remove the class after 200ms

    // If the video element exists and a volume/mute button is clicked
    if (videoElement && ['volume-up', 'volume-down', 'mute'].includes(buttonId)) {
      switch (buttonId) {
        case 'volume-up':
          videoElement.volume = Math.min(1, videoElement.volume + 0.1);
          videoElement.muted = false; // Unmute when volume is changed
          break;
        case 'volume-down':
          videoElement.volume = Math.max(0, videoElement.volume - 0.1);
          videoElement.muted = false; // Unmute when volume is changed
          break;
        case 'mute':
          videoElement.muted = !videoElement.muted;
          break;
      }
      // No need to call updateStatus() for local volume changes
      updateMuteButtonStatus(); // Update the button's class immediately
    } else if (buttonId === "ch") {
      await channelInput();
    } else {
      // For all other buttons, send the command to the backend
      await sendCmd(buttonId);
      updateStatus(); // Update Harmony status after sending a command
    }
  });

  channel.addEventListener('keypress', (event) => {
    if (event.key === "Enter") {
      channelInput();
    }
  });

  pwrButton.addEventListener('click', async () => {
    await sendCmd('power');
    updateStatus();
  });
}

export async function updateStatus() {
  const status = await checkStatus();
  const isOff = status.off !== false;

  toggleRemoteEnabled(!isOff);
  pwrButton.classList.toggle("btn-success", isOff);
  pwrButton.classList.toggle("btn-danger", !isOff);
  pwrButton.ariaPressed = !isOff ? "true" : "false";

  // Also update the mute button status
  updateMuteButtonStatus();
}
