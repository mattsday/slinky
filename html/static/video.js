export function initVideoPlayer() {
  if (!mpegts.isSupported()) {
    console.error("mpegts.js is not supported in this browser.");
    return;
  }

  const videoElement = document.getElementById('slinky-video');
  const sourceDropdown = document.getElementById("quality");
  const videoWrapper = document.getElementById('video-wrapper');
  const videoContainer = document.getElementById('video-container');
  const remotePanel = document.getElementById('remote-panel');
  const fullscreenButton = document.getElementById('fullscreen-button');
  let hideTimeout;
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

  if (sourceDropdown) {
    sourceDropdown.addEventListener('change', (event) => {
      changeStream(event.target.value);
    });
    changeStream(sourceDropdown.value);
  }

  videoElement.addEventListener('click', () => {
    if (player) {
      if (videoElement.paused) {
        player.play();
      } else {
        player.pause();
      }
    }
  });

  if (fullscreenButton && videoWrapper) {
    fullscreenButton.addEventListener('click', () => {
      if (!document.fullscreenElement && !document.webkitFullscreenElement) {
        if (videoWrapper.requestFullscreen) {
          videoWrapper.requestFullscreen();
        } else if (videoWrapper.webkitRequestFullscreen) { /* Safari */
          videoWrapper.webkitRequestFullscreen();
        } else if (videoWrapper.msRequestFullscreen) { /* IE11 */
          videoWrapper.msRequestFullscreen();
        }
      } else {
        if (document.exitFullscreen) {
          document.exitFullscreen();
        } else if (document.webkitExitFullscreen) { /* Safari */
          document.webkitExitFullscreen();
        } else if (document.msExitFullscreen) { /* IE11 */
          document.msExitFullscreen();
        }
      }
    });
  }

  function showMainFullscreenButton() {
    fullscreenButton.classList.add('visible');
  }
  function hideMainFullscreenButton() {
    fullscreenButton.classList.remove('visible');
  }

  videoContainer.addEventListener('mouseenter', showMainFullscreenButton);
  videoContainer.addEventListener('mouseleave', hideMainFullscreenButton);


  function handleFullscreenActivity() {
    // Show controls and cursor
    fullscreenButton.classList.add('visible');
    remotePanel.classList.remove('remote-hidden');
    videoWrapper.classList.remove('cursor-hidden');

    // Reset hide timer
    clearTimeout(hideTimeout);

    hideTimeout = setTimeout(() => {
      fullscreenButton.classList.remove('visible');
      remotePanel.classList.add('remote-hidden');
      videoWrapper.classList.add('cursor-hidden');
    }, 3000); // Hide everything after 3 seconds
  }

  function handleFullscreenChange() {
    const icon = fullscreenButton.querySelector('i');
    if (document.fullscreenElement || document.webkitFullscreenElement) {
      videoWrapper.classList.add('fullscreen-active');
      videoWrapper.classList.remove('video-layout');
      remotePanel.classList.add('remote-hidden');
      videoContainer.removeEventListener('mouseenter', showMainFullscreenButton);
      videoContainer.removeEventListener('mouseleave', hideMainFullscreenButton);
      videoWrapper.addEventListener('mousemove', handleFullscreenActivity);
      videoWrapper.addEventListener('touchstart', handleFullscreenActivity);
      icon.classList.remove('fa-expand');
      icon.classList.add('fa-compress');
    } else {
      videoWrapper.classList.remove('fullscreen-active');
      videoWrapper.classList.add('video-layout');
      remotePanel.classList.remove('remote-hidden');
      videoWrapper.removeEventListener('mousemove', handleFullscreenActivity);
      videoWrapper.removeEventListener('touchstart', handleFullscreenActivity);
      videoContainer.addEventListener('mouseenter', showMainFullscreenButton);
      videoContainer.addEventListener('mouseleave', hideMainFullscreenButton);

      // Clean up state on exit
      fullscreenButton.classList.remove('visible');
      videoWrapper.classList.remove('cursor-hidden');
      clearTimeout(hideTimeout);
      icon.classList.remove('fa-compress');
      icon.classList.add('fa-expand');
    }
  }

  document.addEventListener('fullscreenchange', handleFullscreenChange);
  document.addEventListener('webkitfullscreenchange', handleFullscreenChange);
}

export function initCasting() {
  const video = document.getElementById('slinky-video');
  const castButton = document.getElementById('castButton');
  if (!castButton || !video) return;

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
