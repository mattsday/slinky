export function initVideoPlayer() {
  const videoElement = document.getElementById('slinky-video');
  const sourceDropdown = document.getElementById("quality");
  const videoWrapper = document.getElementById('video-wrapper');
  const videoContainer = document.getElementById('video-container');
  const remotePanel = document.getElementById('remote-panel');
  const fullscreenButton = document.getElementById('fullscreen-button');
  const qualityDisplay = document.getElementById('quality-display');

  let hideTimeout;
  let hlsPlayer;
  let mpegtsPlayer;

  const destroyPlayers = () => {
    if (hlsPlayer) {
      hlsPlayer.destroy();
      hlsPlayer = null;
    }
    if (mpegtsPlayer) {
      mpegtsPlayer.destroy();
      mpegtsPlayer = null;
    }
  };

  const changeStream = (url, name = "") => {
    destroyPlayers();

    if (url === 'hls') {
      if (Hls.isSupported()) {
        hlsPlayer = new Hls();
        hlsPlayer.loadSource('/playlist.m3u8');
        hlsPlayer.attachMedia(videoElement);
        hlsPlayer.on(Hls.Events.MANIFEST_PARSED, () => {
          videoElement.play();
        });
        hlsPlayer.on(Hls.Events.LEVEL_SWITCHED, (event, data) => {
          const level = hlsPlayer.levels[data.level];
          if (level && level.name) {
            qualityDisplay.textContent = `Auto quality: ${level.name}`;
          }
        });
      } else if (videoElement.canPlayType('application/vnd.apple.mpegurl')) {
        videoElement.src = '/playlist.m3u8';
        videoElement.addEventListener('loadedmetadata', () => {
          videoElement.play();
        });
      }
    } else {
      if (mpegts.isSupported()) {
        mpegtsPlayer = mpegts.createPlayer({
          type: 'mpegts',
          isLive: true,
          url: url
        });
        qualityDisplay.textContent = `${name}`;
        mpegtsPlayer.attachMediaElement(videoElement);
        mpegtsPlayer.load();
        mpegtsPlayer.play();
      }
    }
  };

  if (sourceDropdown) {
    // Load last saved quality from localStorage
    const savedQuality = localStorage.getItem('selectedQuality');
    if (savedQuality) {
      sourceDropdown.value = savedQuality;
    }

    sourceDropdown.addEventListener('change', (event) => {
      const selectedValue = event.target.value;
      localStorage.setItem('selectedQuality', selectedValue);
      const name = event.target.options[event.target.selectedIndex].text;
      changeStream(selectedValue, name);
    });
    const name = sourceDropdown.options[sourceDropdown.selectedIndex].text;
    // Initialize with the current value of the dropdown
    changeStream(sourceDropdown.value, name);
  }

  videoElement.addEventListener('click', () => {
    if (videoElement.paused) {
      videoElement.play();
    } else {
      videoElement.pause();
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
    qualityDisplay.classList.add('visible');
  }
  function hideMainFullscreenButton() {
    fullscreenButton.classList.remove('visible');
    qualityDisplay.classList.remove('visible');
  }

  videoContainer.addEventListener('mouseenter', showMainFullscreenButton);
  videoContainer.addEventListener('mouseleave', hideMainFullscreenButton);


  function handleFullscreenActivity() {
    // Show controls and cursor
    fullscreenButton.classList.add('visible');
    remotePanel.classList.remove('remote-hidden');
    videoWrapper.classList.remove('cursor-hidden');
    qualityDisplay.classList.add('visible');

    // Reset hide timer
    clearTimeout(hideTimeout);

    hideTimeout = setTimeout(() => {
      fullscreenButton.classList.remove('visible');
      remotePanel.classList.add('remote-hidden');
      videoWrapper.classList.add('cursor-hidden');
      qualityDisplay.classList.remove('visible');
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
