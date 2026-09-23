/** @odoo-module **/

import { Plugin } from "@html_editor/plugin";
import { registry } from "@web/core/registry";
import { BaseOptionComponent } from "@html_builder/core/utils";
import { BuilderAction } from "@html_builder/core/builder_action";
import { onWillStart, useState } from "@odoo/owl";

export class MachineCatalogSearchSnippetOption extends BaseOptionComponent {
    static template = "website_vegusa.MachineCatalogSearchSnippetOption";
    static selector = ".o_machine_catalog_search_snippet";
    static dependencies = ["machineCatalogSearchSnippetOptionPlugin"];

    setup() {
        super.setup();
        this.state = useState({
            categories: [],
        });

        onWillStart(async () => {
            this.state.categories.push(...(await this.dependencies.machineCatalogSearchSnippetOptionPlugin.fetchCategories()));
        });
    }
}

class MachineCatalogSearchSnippetOptionPlugin extends Plugin {
    static id = "machineCatalogSearchSnippetOptionPlugin";
    static shared = ["fetchCategories"];

    resources = {
        builder_options: [MachineCatalogSearchSnippetOption],
        builder_actions: {
            SetMachineCatalogCategoryAction,
            SetMachineCatalogModeAction,
        },
    };

    async fetchCategories() {
        if (!this.categories) {
            this.categories = this.services.orm.searchRead(
                "product.public.category",
                [["has_published_products", "=", true]],
                ["id", "name", "display_name"],
                { order: "name asc" },
            );
        }
        return this.categories;
    }
}

export class SetMachineCatalogCategoryAction extends BuilderAction {
    static id = "setMachineCatalogCategory";
    static dependencies = ["machineCatalogSearchSnippetOptionPlugin"];

    apply({ editingElement, value }) {
        const hiddenCategory = editingElement.querySelector("input[name='category']");
        if (!hiddenCategory) {
            return;
        }

        const normalizedValue = value || "";
        hiddenCategory.value = normalizedValue;
        hiddenCategory.setAttribute("value", normalizedValue);
        hiddenCategory.disabled = !normalizedValue;
        editingElement.dataset.categoryId = normalizedValue;
    }
}

export class SetMachineCatalogModeAction extends BuilderAction {
    static id = "setMachineCatalogMode";

    apply({ editingElement, value }) {
        // both / vehicle / machinery — read at runtime by machine_search_snippet.js
        editingElement.dataset.searchMode = value || "both";
    }
}

registry.category("builder-plugins").add(MachineCatalogSearchSnippetOptionPlugin.id, MachineCatalogSearchSnippetOptionPlugin);