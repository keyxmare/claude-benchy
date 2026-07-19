(function () {
  var dialog = document.getElementById('cfg-export');
  if (!dialog) { return; }
  var list = document.getElementById('cfg-files');
  var artifact = document.getElementById('cfg-artifact');
  var title = document.getElementById('cfg-title');
  var selectAll = document.getElementById('cfg-selectall');
  var form = document.getElementById('cfg-form');
  var dirValue = form.querySelector('[name=dir]').value;

  document.addEventListener('click', function (e) {
    var btn = e.target.closest('.export-config');
    if (!btn) { return; }
    artifact.value = btn.dataset.artifact;
    title.textContent = 'Exporter la conf — ' + btn.dataset.label;
    list.innerHTML = '<li class="muted">Chargement…</li>';
    selectAll.checked = true;
    fetch('/config-files?dir=' + encodeURIComponent(dirValue) + '&artifact=' + encodeURIComponent(btn.dataset.artifact))
      .then(function (r) { return r.ok ? r.json() : r.text().then(function (t) { throw new Error(t); }); })
      .then(function (data) {
        list.innerHTML = '';
        (data.files || []).forEach(function (f) {
          var li = document.createElement('li');
          var lab = document.createElement('label');
          var cb = document.createElement('input');
          cb.type = 'checkbox'; cb.name = 'files'; cb.value = f; cb.checked = true;
          lab.appendChild(cb); lab.appendChild(document.createTextNode(' ' + f));
          li.appendChild(lab); list.appendChild(li);
        });
      })
      .catch(function (err) { list.innerHTML = '<li class="err">' + (err.message || 'erreur') + '</li>'; });
    dialog.showModal();
  });

  selectAll.addEventListener('change', function () {
    list.querySelectorAll('input[name=files]').forEach(function (cb) { cb.checked = selectAll.checked; });
  });
  document.getElementById('cfg-close').addEventListener('click', function () { dialog.close(); });
  form.addEventListener('submit', function (e) {
    var n = list.querySelectorAll('input[name=files]:checked').length;
    if (n === 0) { e.preventDefault(); return; }
    if (!confirm('Exporter ' + n + ' fichier(s) de conf sur le projet testé ? Les fichiers existants seront écrasés (non committé).')) {
      e.preventDefault();
    }
  });
})();
