/* Indicador de scroll — Refacciones Vegusa
   Dots verticales que sustituyen la scrollbar nativa (oculta vía CSS).
   Se muestran al cambiar de sección o al acercar el mouse al borde
   derecho, y se ocultan solos tras un momento de inactividad.
   No se inicializa en touch/móvil: ahí no hay scrollbar que sustituir. */
(function () {
  'use strict';

  if (!window.matchMedia('(hover: hover) and (pointer: fine)').matches) return;

  var scrollArea = document.querySelector('.site-scroll');
  if (!scrollArea) return;

  /* El footer no es una "sección" de contenido navegable: se excluye
     de los dots (además, su altura variable hace que rara vez cruce
     el umbral de cambio de sección activa). */
  var sections = Array.prototype.slice.call(scrollArea.querySelectorAll(':scope > section[data-name]'));
  if (sections.length < 2) return;

  var LABELS = {
    'Hero Bento Grid': 'Inicio',
    'Categorías (círculos)': 'Categorías',
    'Cómo comprar': '¿Cómo comprar?'
  };
  function labelFor(section) {
    var name = section.dataset.name;
    return LABELS[name] || name;
  }

  var nav = document.createElement('nav');
  nav.className = 'scroll-dots';
  nav.setAttribute('aria-label', 'Indicador de posición en la página');

  var list = document.createElement('ul');
  list.style.cssText = 'list-style:none;margin:0;padding:0;display:flex;flex-direction:column;gap:inherit;';
  nav.appendChild(list);

  var items = sections.map(function (section, i) {
    if (!section.id) section.id = 'scroll-section-' + i;

    var li = document.createElement('li');
    li.className = 'scroll-dots__item';

    var btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'scroll-dots__btn';
    btn.setAttribute('aria-label', 'Ir a ' + labelFor(section));
    btn.addEventListener('click', function () {
      section.scrollIntoView({ behavior: 'smooth', block: 'start' });
    });

    var label = document.createElement('span');
    label.className = 'scroll-dots__label';
    label.textContent = labelFor(section);
    label.setAttribute('aria-hidden', 'true');

    li.appendChild(btn);
    li.appendChild(label);
    list.appendChild(li);
    return li;
  });

  document.body.appendChild(nav);
  items[0].classList.add('is-active');

  var activeIndex = 0;
  var hideTimer = null;
  var hovering = false;
  var ticking = false;

  function setActive(index) {
    if (index === activeIndex) return;
    items[activeIndex].classList.remove('is-active');
    items[index].classList.add('is-active');
    activeIndex = index;
  }

  function scheduleHide() {
    clearTimeout(hideTimer);
    hideTimer = setTimeout(function () {
      if (!hovering) nav.classList.remove('is-visible');
    }, 1400);
  }

  function reveal() {
    nav.classList.add('is-visible');
    scheduleHide();
  }

  function updateFromScroll() {
    ticking = false;
    var areaTop = scrollArea.getBoundingClientRect().top;
    var refY = areaTop + scrollArea.clientHeight * 0.3;
    var current = 0;
    for (var i = 0; i < sections.length; i++) {
      if (sections[i].getBoundingClientRect().top <= refY) current = i;
    }
    setActive(current);
    reveal();
  }

  scrollArea.addEventListener('scroll', function () {
    if (!ticking) {
      ticking = true;
      requestAnimationFrame(updateFromScroll);
    }
  }, { passive: true });

  nav.addEventListener('mouseenter', function () {
    hovering = true;
    clearTimeout(hideTimer);
    nav.classList.add('is-visible');
  });
  nav.addEventListener('mouseleave', function () {
    hovering = false;
    scheduleHide();
  });

  var EDGE_THRESHOLD = 96;
  window.addEventListener('mousemove', function (e) {
    if (window.innerWidth - e.clientX <= EDGE_THRESHOLD) reveal();
  });

  updateFromScroll();
})();
