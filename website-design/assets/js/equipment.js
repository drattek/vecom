/* Mi garage — selección de equipo
   Al elegir un equipo en el side "Mis equipos", el botón del header pasa de
   "Mi garage" a "<modelo> <año>" (el año solo si el equipo lo tiene).
   Volver a tocar el equipo seleccionado lo deselecciona. */
(function () {
  'use strict';

  var list = document.querySelector('[data-equipment-list]');
  var label = document.querySelector('[data-equipment-label]');
  if (!list || !label) return;

  var btn = label.closest('button');
  var panel = list.closest('[data-offcanvas]');
  var cards = Array.prototype.slice.call(list.querySelectorAll('.vehicle-card'));
  var defaultText = label.textContent;
  var defaultTitle = btn ? btn.getAttribute('title') : '';

  function select(card) {
    cards.forEach(function (c) {
      var on = c === card;
      c.classList.toggle('is-selected', on);
      c.setAttribute('aria-pressed', on ? 'true' : 'false');
    });

    if (!card) {
      label.textContent = defaultText;
      if (btn) btn.setAttribute('title', defaultTitle);
      return;
    }

    var d = card.dataset;
    label.textContent = d.year ? d.model + ' ' + d.year : d.model;
    if (btn) btn.setAttribute('title', [d.brand, d.model, d.year].filter(Boolean).join(' '));
  }

  cards.forEach(function (card) {
    card.addEventListener('click', function () {
      select(card.classList.contains('is-selected') ? null : card);
      var close = panel && panel.querySelector('[data-offcanvas-close]');
      if (close) close.click();
    });
  });
})();
