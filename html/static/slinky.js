import { initRemoteControls, updateStatus } from './remote.js';
import { initVideoPlayer, initCasting } from './video.js';

document.addEventListener('DOMContentLoaded', () => {
    initRemoteControls();
    initVideoPlayer();
    initCasting();

    // Perform initial status check
    updateStatus();
    // Set up the recurring status check
    setInterval(updateStatus, 10000);

    document.addEventListener('keydown', (event) => {
        if (event.target.tagName === 'INPUT') {
            return; // Don't do anything if the user is typing in an input field
        }

        let buttonId;
        switch (event.code) {
            case 'Space':
                event.preventDefault();
                buttonId = 'play';
                break;
            case 'ArrowUp':
                event.preventDefault();
                buttonId = 'volume-up';
                break;
            case 'ArrowDown':
                event.preventDefault();
                buttonId = 'volume-down';
                break;
            case 'ArrowLeft':
                event.preventDefault();
                buttonId = 'channel-down';
                break;
            case 'ArrowRight':
                event.preventDefault();
                buttonId = 'channel-up';
                break;
            case 'KeyM':
                buttonId = 'mute';
                break;
            case 'KeyF':
                event.preventDefault();
                buttonId = 'fullscreen-button';
                break;
        }

        if (buttonId) {
            const button = document.getElementById(buttonId);
            if (button) {
                button.click();
            }
        }
    });
});
