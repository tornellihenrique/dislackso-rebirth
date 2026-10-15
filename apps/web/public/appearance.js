/* global document */
// Applies the saved theme and density before the first paint, so a light
// theme never flashes dark. Mirrors appearanceStorage in src/stores/appearance.ts.
(function () {
  try {
    var saved = JSON.parse(localStorage.getItem('dsx.appearance') || 'null');
    if (!saved || saved.v !== 1 || !saved.data) return;
    var root = document.documentElement;
    if (['dark', 'onyx', 'light'].indexOf(saved.data.theme) >= 0)
      root.dataset.theme = saved.data.theme;
    if (['compact', 'default', 'spacious'].indexOf(saved.data.density) >= 0) {
      root.dataset.density = saved.data.density;
    }
  } catch {
    // Unreadable storage: keep the defaults from index.html.
  }
})();
