const API_ROOT = "/api/v1";
let statusController; // To hold the AbortController for status checks

export async function checkStatus() {
  // If there's an ongoing request, abort it.
  if (statusController) {
    statusController.abort();
  }
  statusController = new AbortController();
  const signal = statusController.signal;

  try {
    const response = await fetch(`${API_ROOT}/pwr`, { signal });
    if (!response.ok) {
      throw new Error(`Server responded with ${response.status}`);
    }
    return await response.json();
  } catch (error) {
    if (error.name === 'AbortError') {
      console.log('Previous status fetch aborted.');
    } else {
      console.error("Failed to check power status:", error);
    }
    return { off: true }; // Assume off on error
  } finally {
    statusController = null; // Clear the controller
  }
}

export async function sendCmd(cmd) {
  try {
    await fetch(`${API_ROOT}/call/${cmd}`);
  } catch (error) {
    console.error(`Failed to send command ${cmd}:`, error);
  }
}
