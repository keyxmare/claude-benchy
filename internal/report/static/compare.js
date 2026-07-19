(function () {
  var data = BENCHY_FILES;
  var labels = data.labels || [];
  var left = document.getElementById('sxs-left');
  var right = document.getElementById('sxs-right');
  var body = document.getElementById('sxs-body');

  if (labels.length < 2) {
    body.innerHTML = '<p class="empty">Comparaison indisponible : moins de deux configurations ont produit des fichiers.</p>';
    var controls = document.querySelector('.sxs-controls');
    if (controls) { controls.style.display = 'none'; }
    return;
  }

  labels.forEach(function (l) {
    left.add(new Option(l, l));
    right.add(new Option(l, l));
  });
  left.value = labels[0];
  right.value = labels[1];

  function esc(s) {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function toLines(s) {
    if (!s) { return []; }
    var a = s.split('\n');
    if (a.length && a[a.length - 1] === '') { a.pop(); }
    return a;
  }

  function lcsOps(a, b) {
    var n = a.length, m = b.length, ops = [], i, j;
    if (n * m > 4000000) {
      for (i = 0; i < n; i++) { ops.push({ t: 'del', i: i }); }
      for (j = 0; j < m; j++) { ops.push({ t: 'ins', j: j }); }
      return ops;
    }
    var dp = [];
    for (i = 0; i <= n; i++) { dp[i] = new Int32Array(m + 1); }
    for (i = n - 1; i >= 0; i--) {
      for (j = m - 1; j >= 0; j--) {
        dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
      }
    }
    i = 0; j = 0;
    while (i < n && j < m) {
      if (a[i] === b[j]) { ops.push({ t: 'eq', i: i, j: j }); i++; j++; }
      else if (dp[i + 1][j] >= dp[i][j + 1]) { ops.push({ t: 'del', i: i }); i++; }
      else { ops.push({ t: 'ins', j: j }); j++; }
    }
    while (i < n) { ops.push({ t: 'del', i: i }); i++; }
    while (j < m) { ops.push({ t: 'ins', j: j }); j++; }
    return ops;
  }

  function alignRows(a, b) {
    var ops = lcsOps(a, b), rows = [], ln = 0, rn = 0, pd = [], pi = [];
    function flush() {
      var k = Math.max(pd.length, pi.length), x;
      for (x = 0; x < k; x++) {
        var L = x < pd.length ? { n: ++ln, text: a[pd[x]] } : null;
        var R = x < pi.length ? { n: ++rn, text: b[pi[x]] } : null;
        rows.push({ l: L, r: R, cls: (L && R) ? 'chg' : (L ? 'del' : 'add') });
      }
      pd = []; pi = [];
    }
    ops.forEach(function (op) {
      if (op.t === 'eq') {
        flush();
        rows.push({ l: { n: ++ln, text: a[op.i] }, r: { n: ++rn, text: b[op.j] }, cls: 'eq' });
      } else if (op.t === 'del') { pd.push(op.i); }
      else { pi.push(op.j); }
    });
    flush();
    return rows;
  }

  function cell(side, c) {
    if (!c) { return '<td class="ln"></td><td class="code ' + side + '"></td>'; }
    return '<td class="ln">' + c.n + '</td><td class="code ' + side + '">' + esc(c.text) + '</td>';
  }

  function badge(inL, inR, same) {
    var txt = !inL ? 'droite seulement' : !inR ? 'gauche seulement' : same ? 'identique' : 'divergent';
    return '<span class="ftag">' + txt + '</span>';
  }

  function render() {
    var lf = data.files[left.value] || {}, rf = data.files[right.value] || {};
    var seen = {};
    Object.keys(lf).forEach(function (k) { seen[k] = 1; });
    Object.keys(rf).forEach(function (k) { seen[k] = 1; });
    var keys = Object.keys(seen).sort();
    if (!keys.length) { body.innerHTML = '<p class="empty">Aucun fichier pour ce couple.</p>'; return; }

    var out = '';
    keys.forEach(function (path) {
      var inL = Object.prototype.hasOwnProperty.call(lf, path);
      var inR = Object.prototype.hasOwnProperty.call(rf, path);
      var lc = inL ? lf[path] : '', rc = inR ? rf[path] : '';
      out += '<div class="sxs-file">';
      out += '<div class="sxs-head"><code>' + esc(path) + '</code>' + badge(inL, inR, lc === rc) + '</div>';
      out += '<div class="sxs-scroll"><table class="sxs"><tbody>';
      alignRows(toLines(lc), toLines(rc)).forEach(function (row) {
        out += '<tr class="' + row.cls + '">' + cell('left', row.l) + cell('right', row.r) + '</tr>';
      });
      out += '</tbody></table></div></div>';
    });
    body.innerHTML = out;
  }

  left.addEventListener('change', render);
  right.addEventListener('change', render);
  render();
})();

(function () {
  var links = Array.prototype.slice.call(document.querySelectorAll('#toc-nav a'));
  if (!links.length || !('IntersectionObserver' in window)) { return; }
  var sections = links
    .map(function (a) { return document.getElementById(a.getAttribute('href').slice(1)); })
    .filter(Boolean);
  var current = null;
  function setActive(id) {
    if (id === current) { return; }
    current = id;
    links.forEach(function (a) { a.classList.toggle('active', a.getAttribute('href').slice(1) === id); });
  }
  var observer = new IntersectionObserver(function (entries) {
    entries.forEach(function (e) { if (e.isIntersecting) { setActive(e.target.id); } });
  }, { rootMargin: '0px 0px -75% 0px', threshold: 0 });
  sections.forEach(function (s) { observer.observe(s); });
})();
