/** @odoo-module **/

import publicWidget from "@web/legacy/js/public/public_widget";
import { rpc } from "@web/core/network/rpc";

const BLOCK = "o_machine_catalog_search_snippet";
let widgetCounter = 0;

/**
 * Search box that can be dropped on any page. The snippet's saved HTML is only a
 * shell (just the <form>): everything else is built here from the current
 * catalog, so new brands/types created by the core-orchestrator sync show up in
 * snippets that were placed long ago, and the form can evolve without touching
 * the pages that already contain it.
 *
 * Two searches, chosen with a toggle when the snippet is in "both" mode:
 *   - vehicle:   Marca → Modelo → Año (the year is mandatory)
 *   - machinery: Tipo → Marca → Modelo
 * The mode can be pinned per snippet with data-search-mode (both/vehicle/machinery),
 * set from the website builder.
 */
publicWidget.registry.MachineCatalogSearchSnippet = publicWidget.Widget.extend({
    selector: `.${BLOCK}`,

    start() {
        // The legacy Class only exposes this._super during the synchronous part of the
        // method, so it must be called before any await.
        const superPromise = this._super(...arguments);
        return Promise.all([superPromise, this._init()]);
    },

    async _init() {
        this.uid = ++widgetCounter;
        this.requestToken = 0;
        this.catalog = await rpc("/website_vegusa/machine_catalog/search_types", {});
        this._build();
    },

    // ------------------------------------------------------------------
    // Layout
    // ------------------------------------------------------------------

    _availableKinds() {
        const mode = this.el.dataset.searchMode || "both";
        const kinds = [];
        if (this.catalog.vehicle_type_id && mode !== "machinery") {
            kinds.push({ id: "vehicle", label: "Vehículo" });
        }
        if (this.catalog.machine_types.length && mode !== "vehicle") {
            kinds.push({ id: "machinery", label: "Maquinaria" });
        }
        return kinds;
    },

    _build() {
        const form = this.el.querySelector("form");
        if (!form) {
            return;
        }
        // The title and subtitle were removed from the design; drop them from snippets
        // placed before that (their HTML is static).
        this.el.querySelectorAll(`.${BLOCK}__title, .${BLOCK}__subtitle`).forEach((element) => element.remove());
        // Keep the category hidden input the builder option writes to; rebuild the rest.
        let category = form.querySelector("input[name='category']");
        form.replaceChildren();
        if (!category) {
            category = document.createElement("input");
            category.type = "hidden";
            category.name = "category";
            category.value = this.el.dataset.categoryId || "";
            category.disabled = !category.value;
        }
        form.appendChild(category);

        this.kinds = this._availableKinds();
        if (!this.kinds.length) {
            const message = document.createElement("p");
            message.className = "mb-0";
            message.textContent = "La búsqueda no está disponible por el momento.";
            form.appendChild(message);
            return;
        }

        if (this.kinds.length > 1) {
            this.toggle = this._buildToggle();
            form.appendChild(this.toggle);
        }
        this.fields = document.createElement("div");
        this.fields.className = "row g-3 align-items-end";
        form.appendChild(this.fields);

        this._setKind(this.kinds[0].id);
    },

    _buildToggle() {
        const group = document.createElement("div");
        group.className = "btn-group mb-3";
        group.setAttribute("role", "group");
        for (const kind of this.kinds) {
            const button = document.createElement("button");
            button.type = "button";
            button.className = "btn btn-outline-secondary";
            button.dataset.kind = kind.id;
            button.textContent = kind.label;
            button.addEventListener("click", () => this._setKind(kind.id));
            group.appendChild(button);
        }
        return group;
    },

    _setKind(kind) {
        this.kind = kind;
        this.toggle?.querySelectorAll("button").forEach((button) => {
            const active = button.dataset.kind === kind;
            button.classList.toggle("active", active);
            button.setAttribute("aria-pressed", active);
        });
        this._renderFields();
    },

    _renderFields() {
        this.requestToken++;
        this.fields.replaceChildren();
        const isVehicle = this.kind === "vehicle";

        this.typeSelect = null;
        if (isVehicle) {
            const typeInput = document.createElement("input");
            typeInput.type = "hidden";
            typeInput.name = "machine_type";
            typeInput.value = this.catalog.vehicle_type_id;
            this.fields.appendChild(typeInput);
            this.machineTypeId = this.catalog.vehicle_type_id;
        } else {
            this.machineTypeId = null;
            this.typeSelect = this._select("machine_type", "Selecciona el tipo");
            this._fillSelect(
                this.typeSelect,
                this.catalog.machine_types.map((type) => ({ value: type.id, label: type.name })),
                "Selecciona el tipo"
            );
            this.typeSelect.addEventListener("change", () => this._onTypeChange());
            this._addField("Tipo", this.typeSelect);
        }

        this.brandSelect = this._select("brand", "Selecciona una marca", true);
        this.brandSelect.addEventListener("change", () => this._onBrandChange());
        this._addField("Marca", this.brandSelect);

        this.modelSelect = this._select("machine_model_id", "Selecciona el modelo", true);
        this.modelSelect.addEventListener("change", () => this._onModelChange());
        this._addField("Modelo", this.modelSelect);

        this.yearSelect = null;
        if (isVehicle) {
            this.yearSelect = this._select("vehicle_year", "Selecciona el modelo primero", true);
            this._addField("Año", this.yearSelect);
        }

        const submitWrap = document.createElement("div");
        submitWrap.className = "col-12 col-lg-3 ms-lg-auto";
        const submit = document.createElement("button");
        submit.type = "submit";
        submit.className = `btn ${BLOCK}__button`;
        submit.textContent = "Buscar";
        submitWrap.appendChild(submit);
        this.fields.appendChild(submitWrap);

        if (isVehicle) {
            this._loadBrands();
        } else {
            this._setDisabledHint(this.brandSelect, "Selecciona el tipo primero");
        }
        this._setDisabledHint(this.modelSelect, "Selecciona la marca primero");
    },

    // ------------------------------------------------------------------
    // Cascade
    // ------------------------------------------------------------------

    _onTypeChange() {
        this.machineTypeId = parseInt(this.typeSelect.value, 10) || null;
        this._setDisabledHint(this.modelSelect, "Selecciona la marca primero");
        if (this.machineTypeId) {
            this._loadBrands();
        } else {
            this._setDisabledHint(this.brandSelect, "Selecciona el tipo primero");
        }
    },

    async _loadBrands() {
        const token = ++this.requestToken;
        const brands = await rpc("/website_vegusa/machine_catalog/search_brands", {
            machine_type_id: this.machineTypeId,
        });
        if (token !== this.requestToken) {
            return;
        }
        this._fillSelect(
            this.brandSelect,
            brands.map((brand) => ({ value: brand.id, label: brand.name })),
            "Selecciona una marca"
        );
    },

    async _onBrandChange() {
        const brandId = parseInt(this.brandSelect.value, 10);
        this.models = [];
        if (this.yearSelect) {
            this._setDisabledHint(this.yearSelect, "Selecciona el modelo primero");
        }
        if (!brandId) {
            this._setDisabledHint(this.modelSelect, "Selecciona la marca primero");
            return;
        }
        const token = ++this.requestToken;
        const result = await rpc("/website_vegusa/machine_catalog/models", {
            brand_id: brandId,
            machine_type_id: this.machineTypeId,
        });
        if (token !== this.requestToken) {
            return;
        }
        this.models = result?.models || [];
        this._fillSelect(
            this.modelSelect,
            this.models.map((model) => ({ value: model.id, label: model.name })),
            "Selecciona el modelo"
        );
    },

    _onModelChange() {
        if (!this.yearSelect) {
            return;
        }
        const model = this.models.find((candidate) => String(candidate.id) === this.modelSelect.value);
        if (!model) {
            this._setDisabledHint(this.yearSelect, "Selecciona el modelo primero");
            return;
        }
        this._fillSelect(
            this.yearSelect,
            (model.years || []).map((year) => ({ value: year, label: String(year) })),
            "Selecciona el año"
        );
        // The year is mandatory for vehicles: browsers block the submit until picked.
        this.yearSelect.required = true;
    },

    // ------------------------------------------------------------------
    // DOM helpers
    // ------------------------------------------------------------------

    _select(name, placeholder, disabled = false) {
        const select = document.createElement("select");
        select.id = `machine_catalog_${this.uid}_${name}`;
        select.name = name;
        select.className = `form-select ${BLOCK}__select`;
        select.disabled = disabled;
        this._fillSelect(select, [], placeholder);
        select.disabled = disabled;
        return select;
    },

    _fillSelect(select, options, placeholder) {
        select.replaceChildren(new Option(placeholder, ""));
        for (const option of options) {
            select.appendChild(new Option(option.label, option.value));
        }
        select.disabled = !options.length;
        select.required = false;
    },

    _setDisabledHint(select, hint) {
        this._fillSelect(select, [], hint);
    },

    _addField(labelText, select) {
        const wrap = document.createElement("div");
        wrap.className = "col-12 col-lg-3";
        const label = document.createElement("label");
        label.className = `${BLOCK}__label`;
        label.htmlFor = select.id;
        label.textContent = labelText;
        wrap.append(label, select);
        this.fields.appendChild(wrap);
    },
});
