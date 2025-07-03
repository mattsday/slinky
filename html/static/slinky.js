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
});
