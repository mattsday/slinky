import { sendCmd, checkStatus } from './api.js';

const pwrButton = document.getElementById("power");
const remoteCtrl = document.getElementById('remote');
const channel = document.getElementById("channel");

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

export function initRemoteControls() {
  if (!remoteCtrl) return;

  remoteCtrl.addEventListener('click', async (event) => {
    const button = event.target.closest('button');
    if (!button || button.disabled) return;

    if (button.id === "ch") {
      await channelInput();
    } else {
      await sendCmd(button.id);
    }
    updateStatus(); // Update status after every command
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
}
