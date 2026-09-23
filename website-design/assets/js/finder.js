/* Buscador (estilo garage) — Refacciones Vegusa
   Pestañas Maquinaria / Vehículo: alternan los campos visibles del
   formulario. El año de "Vehículo" se genera dinámicamente. */
(function () {
  'use strict';

  var panel = document.querySelector('[data-finder]');
  if (!panel) return;

  var tabs = panel.querySelectorAll('[data-finder-tab]');
  var fieldGroups = panel.querySelectorAll('[data-finder-fields]');

  function activate(type) {
    tabs.forEach(function (tab) {
      var isActive = tab.getAttribute('data-finder-tab') === type;
      tab.classList.toggle('is-active', isActive);
      tab.setAttribute('aria-selected', isActive ? 'true' : 'false');
    });
    fieldGroups.forEach(function (group) {
      var isActive = group.getAttribute('data-finder-fields') === type;
      group.classList.toggle('is-hidden', !isActive);
      group.hidden = !isActive;
    });
  }

  tabs.forEach(function (tab) {
    tab.addEventListener('click', function () {
      activate(tab.getAttribute('data-finder-tab'));
    });
  });

  /* Año de vehículo: del año actual (+1, para modelos del año siguiente) hacia atrás */
  var yearSelect = panel.querySelector('[data-finder-year]');
  if (yearSelect) {
    var currentYear = new Date().getFullYear();
    for (var y = currentYear + 1; y >= 1990; y--) {
      var opt = document.createElement('option');
      opt.value = String(y);
      opt.textContent = String(y);
      yearSelect.appendChild(opt);
    }
  }
})();
