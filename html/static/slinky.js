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
        pwrClass.add("btn-danger")
        pwrClass.remove("btn-success")
    } else {
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
        console.log("Returning!")
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

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

window.HELP_IMPROVE_VIDEOJS = false;