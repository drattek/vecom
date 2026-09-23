# Part of Odoo. See LICENSE file for full copyright and licensing details.
{
    'name': 'Machinery and Vehicles Catalog for eCommerce',
    'summary': 'Add machine and vehicle brands, types, models and years for website_sale',
    'description': 'Machine brands, machine types, machine/vehicle models (vehicles carry a year range; the year is mandatory when filtering by a vehicle), product compatibility, website filters and the ecom_sync_fitments entry point used by core-orchestrator.',
    'category': 'Website/Website',
    'version': '1.1.0',
    'license': 'LGPL-3',
    'author': 'Custom',
    'depends': ['website', 'website_sale', 'portal', 'product'],
    'data': [
        'security/ir.model.access.csv',
        'views/product_brand_views.xml',
        'views/product_type_views.xml',
        'views/product_model_views.xml',
        'views/website_brand_templates.xml',
        'views/website_type_templates.xml',
        'views/machine_search_snippet_templates.xml',
        'views/website_filter_templates.xml',
        'views/website_sale_inherit_templates.xml',
        'views/machine_garage_templates.xml',
    ],
    'assets': {
        'web.assets_frontend': [
            'website_sale_machine_catalog/static/src/js/machine_search_snippet.js',
            'website_sale_machine_catalog/static/src/js/machine_filter_checkbox.js',
            'website_sale_machine_catalog/static/src/scss/website_sale_machine_catalog.scss',
        ],
        'website.website_builder_assets': [
            'website_sale_machine_catalog/static/src/builder/plugins/options/machine_search_snippet_option.js',
            'website_sale_machine_catalog/static/src/builder/plugins/options/machine_search_snippet_option.xml',
        ],
    },
    'installable': True,
    'application': False,
    'auto_install': False,
}
