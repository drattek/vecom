/* Header — Refacciones Vegusa
   Side panels (offcanvas) con overlay de fondo.
   Cada botón [data-offcanvas-open="idDelPanel"] abre su panel; sin valor
   abre el primer panel de la página (compatibilidad con index.html). */
(function () {
  'use strict';

  var panels = Array.prototype.slice.call(document.querySelectorAll('[data-offcanvas]'));
  if (!panels.length) return;

  function setup(offcanvas, openers) {
    function open() {
      offcanvas.hidden = false;
      document.body.style.overflow = 'hidden';
      requestAnimationFrame(function () { offcanvas.classList.add('is-open'); });
      openers.forEach(function (b) { b.setAttribute('aria-expanded', 'true'); });
    }

    function close() {
      offcanvas.classList.remove('is-open');
      document.body.style.overflow = '';
      openers.forEach(function (b) { b.setAttribute('aria-expanded', 'false'); });
      setTimeout(function () {
        if (!offcanvas.classList.contains('is-open')) offcanvas.hidden = true;
      }, 250);
    }

    openers.forEach(function (b) { b.addEventListener('click', open); });

    /* ---- Accordion(es) dentro del side (p. ej. "Mi garage" en index.html) ---- */
    offcanvas.querySelectorAll('[data-accordion]').forEach(function (acc) {
      var btn = acc.querySelector('.garage__toggle');
      var body = acc.querySelector('.garage__body');
      if (!btn || !body) return;

      btn.addEventListener('click', function () {
        var isOpen = acc.classList.toggle('is-open');
        btn.setAttribute('aria-expanded', isOpen ? 'true' : 'false');

        if (isOpen) {
          body.style.height = body.scrollHeight + 'px';
          body.addEventListener('transitionend', function done() {
            body.style.height = 'auto';
            body.removeEventListener('transitionend', done);
          });
        } else {
          body.style.height = body.scrollHeight + 'px';   // fijar px antes de colapsar
          requestAnimationFrame(function () { body.style.height = '0px'; });
        }
      });
    });

    offcanvas.querySelectorAll('[data-offcanvas-close]').forEach(function (el) {
      el.addEventListener('click', close);
    });

    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && !offcanvas.hidden) close();
    });
  }

  var buttons = Array.prototype.slice.call(document.querySelectorAll('[data-offcanvas-open]'));
  panels.forEach(function (panel, i) {
    var openers = buttons.filter(function (b) {
      var target = b.getAttribute('data-offcanvas-open');
      return target ? target === panel.id : i === 0;
    });
    if (openers.length) setup(panel, openers);
  });
})();
