import { Plugin } from "@html_editor/plugin";
import { registry } from "@web/core/registry";
import { _t } from "@web/core/l10n/translation";
import { FooterTemplateChoice } from "@website/builder/plugins/options/footer_template_option";

export class VegusaFooterOptionPlugin extends Plugin {
   static id = "vegusaFooterOption";
   resources = {
      footer_templates_providers: () => [
         {
            key: "vegusa",
            Component: FooterTemplateChoice,
            props: {
               title: _t("Vegusa"),
               view: "website_vegusa.footer",
               varName: "vegusa",
               imgSrc: "/website_vegusa/static/src/img/wbuilder/footer_template_vegusa.svg",
            },
         },
      ],
   };
}

registry.category("website-plugins").add(VegusaFooterOptionPlugin.id, VegusaFooterOptionPlugin);