/** @odoo-module **/

import publicWidget from "@web/legacy/js/public/public_widget";

const HIDE_DELAY = 250; // matches the slide-out transition in header.scss

/**
 * Side panels of the Vegusa header (user menu, garage). A panel is `.vg-offcanvas` with an id;
 * every `[data-offcanvas-open="<id>"]` button opens it, and anything with `data-offcanvas-close`
 * inside it (backdrop, close button) or the Escape key closes it.
 * Design reference: website-design/assets/js/header.js.
 *
 * The widget is bound to the header, not to the panels: legacy public widgets only start inside
 * #wrapwrap, and the panels are rendered next to it (body level, so they can sit above the
 * livechat sticky bar — see side_panels in website_templates.xml).
 */
publicWidget.registry.VegusaHeaderPanels = publicWidget.Widget.extend({
    selector: ".vg-header",

    start() {
        // The legacy Class only exposes this._super during the synchronous part of the method.
        const superPromise = this._super(...arguments);
        this.listeners = [];
        this.hideTimeouts = new Map();

        for (const panel of document.querySelectorAll(".vg-offcanvas")) {
            const openers = Array.from(document.querySelectorAll(`[data-offcanvas-open="${panel.id}"]`));
            this._listen(openers, "click", () => this._open(panel, openers));
            this._listen(panel.querySelectorAll("[data-offcanvas-close]"), "click", () => this._close(panel, openers));
            this._listen([document], "keydown", (ev) => {
                if (ev.key === "Escape" && !panel.hidden) {
                    this._close(panel, openers);
                }
            });
        }
        return superPromise;
    },

    destroy() {
        this.listeners.forEach(({ target, type, handler }) => target.removeEventListener(type, handler));
        this.hideTimeouts.forEach((timeout) => clearTimeout(timeout));
        document.body.classList.remove("vg-offcanvas-open");
        this._super(...arguments);
    },

    _listen(targets, type, handler) {
        for (const target of targets) {
            target.addEventListener(type, handler);
            this.listeners.push({ target, type, handler });
        }
    },

    _open(panel, openers) {
        clearTimeout(this.hideTimeouts.get(panel));
        panel.hidden = false;
        document.body.classList.add("vg-offcanvas-open");
        requestAnimationFrame(() => panel.classList.add("is-open"));
        openers.forEach((opener) => opener.setAttribute("aria-expanded", "true"));
    },

    _close(panel, openers) {
        panel.classList.remove("is-open");
        document.body.classList.remove("vg-offcanvas-open");
        openers.forEach((opener) => opener.setAttribute("aria-expanded", "false"));
        // Let the slide-out transition finish before taking the panel out of the layout.
        this.hideTimeouts.set(
            panel,
            setTimeout(() => {
                if (!panel.classList.contains("is-open")) {
                    panel.hidden = true;
                }
            }, HIDE_DELAY)
        );
    },
});
