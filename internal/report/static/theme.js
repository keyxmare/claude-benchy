// A pre-paint snippet inlined in the self-contained report's <head> applies the
// persisted theme; this file only wires the toggle button and keeps its glyph in
// sync.
(function () {
  var KEY = 'benchy-theme';
  var root = document.documentElement;
  var btn = document.getElementById('theme-toggle');
  if (!btn) { return; }

  var mq = window.matchMedia('(prefers-color-scheme: light)');

  function effective() {
    var forced = root.getAttribute('data-theme');
    if (forced) { return forced; }
    return mq.matches ? 'light' : 'dark';
  }

  function paint() {
    var dark = effective() === 'dark';
    btn.textContent = dark ? '☀' : '☾';
    btn.setAttribute('aria-label', dark ? 'Passer au thème clair' : 'Passer au thème sombre');
  }

  btn.addEventListener('click', function () {
    var next = effective() === 'dark' ? 'light' : 'dark';
    root.setAttribute('data-theme', next);
    try { localStorage.setItem(KEY, next); } catch (e) { /* private mode */ }
    paint();
  });

  mq.addEventListener('change', paint);
  paint();
})();
