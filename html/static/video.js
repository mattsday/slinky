export function initVideoPlayer() {
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

  if (sourceDropdown) {
    sourceDropdown.addEventListener('change', (event) => {
      changeStream(event.target.value);
    });

    changeStream(sourceDropdown.value); // Load initial stream
  }
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
