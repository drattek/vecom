{
    'name': 'Vegusa Theme',
    'description': 'Vegusa Theme for Odoo',
    'category': 'Website/Theme',
    'version': '19.0.0',
    'author': 'Vegusa',
    'license': 'LGPL-3',
    'depends': ['website', 'web', 'website_sale', 'im_livechat'],
    'data': [
        'data/website.xml',
        'data/presets.xml',
        'views/website_templates.xml',
        'views/snippets/s_dynamic_wrapper/s_dynamic_wrapper.xml',
        'views/snippets/s_hero_slider/s_hero_slider.xml',
        'views/snippets.xml',
    ],
    'assets': {
        'web.assets_frontend': [
            'website_vegusa/static/src/scss/primary_variables.scss',
            'website_vegusa/static/src/scss/theme.scss',
            'website_vegusa/static/src/js/s_dynamic_wrapper/s_dynamic_wrapper.js',
            'website_vegusa/static/src/snippets/s_hero_slider/000.scss',
        ],
        'web._assets_primary_variables': [
            'website_vegusa/static/src/scss/primary_variables.scss',
        ],
        'website.website_builder_assets': [
            'website_vegusa/static/src/website_builder/**/*',
        ],
        'im_livechat.assets_embed_core': [
            'website_vegusa/static/src/js/livechat_sticky_bar/livechat_sticky_bar.xml',
            'website_vegusa/static/src/js/livechat_sticky_bar/livechat_sticky_bar.js',
            'website_vegusa/static/src/scss/livechat_sticky_bar.scss',
        ],
    },
}
