/* Ficha de producto — Refacciones Vegusa
   Galería de miniaturas · selector de cantidad · favorito */
(function () {
  'use strict';

  /* ---- Galería: click en miniatura cambia la imagen principal ---- */
  var mainImg = document.getElementById('productMainImage');
  var thumbs = document.querySelectorAll('.product__thumb');

  thumbs.forEach(function (btn) {
    btn.addEventListener('click', function () {
      var full = btn.getAttribute('data-full');
      if (!full || !mainImg) return;

      mainImg.src = full;

      var thumbImg = btn.querySelector('img');
      if (thumbImg && thumbImg.alt) mainImg.alt = thumbImg.alt;

      thumbs.forEach(function (t) { t.classList.remove('is-active'); });
      btn.classList.add('is-active');
    });
  });

  /* ---- Selector de cantidad ---- */
  var qtyInput = document.getElementById('productQty');
  var minusBtn = document.querySelector('[data-qty-minus]');
  var plusBtn = document.querySelector('[data-qty-plus]');

  function clamp(value) {
    var n = parseInt(value, 10);
    if (isNaN(n)) n = 1;
    return Math.min(99, Math.max(1, n));
  }

  if (minusBtn && qtyInput) {
    minusBtn.addEventListener('click', function () {
      qtyInput.value = clamp(parseInt(qtyInput.value, 10) - 1);
    });
  }
  if (plusBtn && qtyInput) {
    plusBtn.addEventListener('click', function () {
      qtyInput.value = clamp(parseInt(qtyInput.value, 10) + 1);
    });
  }
  if (qtyInput) {
    qtyInput.addEventListener('change', function () {
      qtyInput.value = clamp(qtyInput.value);
    });
  }

  /* ---- Botón favorito ---- */
  var favBtn = document.querySelector('.btn-fav');
  if (favBtn) {
    favBtn.addEventListener('click', function () {
      var isActive = favBtn.classList.toggle('is-active');
      var icon = favBtn.querySelector('i');
      if (icon) {
        icon.classList.toggle('fa-regular', !isActive);
        icon.classList.toggle('fa-solid', isActive);
      }
      favBtn.setAttribute('aria-pressed', isActive ? 'true' : 'false');
    });
  }

  /* ---- Descripción: "Leer más" / "Ver menos" ---- */
  var descToggle = document.getElementById('descriptionToggle');
  var descContent = document.getElementById('productDescription');
  if (descToggle && descContent) {
    descToggle.addEventListener('click', function () {
      var isExpanded = descContent.classList.toggle('is-expanded');
      descToggle.setAttribute('aria-expanded', isExpanded ? 'true' : 'false');
      descToggle.querySelector('.description__toggle-label').textContent = isExpanded ? 'Ver menos' : 'Leer más';
    });
  }
})();
