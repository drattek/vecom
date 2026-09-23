import { DynamicSnippetProducts } from "@website_sale/snippets/s_dynamic_snippet_products/dynamic_snippet_products";
import { registry } from "@web/core/registry";

export class DynamicWrapperSnippet extends DynamicSnippetProducts {
    static selector = ".s_dynamic_snippet_products";

    /**
     * Override to add stock filter
     */
    getSearchDomain() {
        const searchDomain = super.getSearchDomain(...arguments);
        // Add filter to show only products with available stock
        // Filter through product variants: any variant must have qty_available > 0
        searchDomain.push(["product_variant_ids.qty_available", ">", 0]);
        return searchDomain;
    }
}

registry
    .category("public.interactions")
    .add("website_sale.dynamic_snippet_products", DynamicWrapperSnippet, { force: true });

registry
    .category("public.interactions.edit")
    .add("website_sale.dynamic_snippet_products", {
        Interaction: DynamicWrapperSnippet,
    }, { force: true });
