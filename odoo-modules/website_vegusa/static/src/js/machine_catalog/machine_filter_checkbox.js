/** @odoo-module **/

import publicWidget from "@web/legacy/js/public/public_widget";

/**
 * The shop's machine filters are checkboxes that carry the URL applying (or, when
 * ticked, clearing) them in data-url — see _get_machine_filters: changing one just
 * navigates there.
 */
publicWidget.registry.MachineCatalogFilterCheckbox = publicWidget.Widget.extend({
    selector: ".o_machine_catalog_filter_checkbox",
    events: {
        change: "_onChange",
    },

    _onChange() {
        if (this.el.dataset.url) {
            window.location.href = this.el.dataset.url;
        }
    },
});
