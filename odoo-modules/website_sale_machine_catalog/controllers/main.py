# Part of Odoo. See LICENSE file for full copyright and licensing details.

import itertools
import re
from datetime import datetime
from urllib.parse import urlencode

from odoo import fields, http
from odoo.fields import Domain
from odoo.http import request
from odoo.tools import SQL, float_round, lazy
from odoo.tools.translate import LazyTranslate

from odoo.addons.website.controllers.main import QueryURL
from odoo.addons.website.models.ir_http import sitemap_qs2dom
from odoo.addons.website_sale.controllers.main import TableCompute, WebsiteSale
from odoo.addons.website_sale.const import SHOP_PATH

_lt = LazyTranslate(__name__)


def sitemap_shop_with_catalogs(env, rule, qs):
    yield from WebsiteSale.sitemap_shop(env, rule, qs)

    website = env['website'].get_current_website()
    if website and website.ecommerce_access == 'logged_in' and not qs:
        return

    Brand = env['machine.brand']
    dom = sitemap_qs2dom(qs, f'{SHOP_PATH}/brand', Brand._rec_name)
    dom &= Domain('website_published', '=', True)
    for brand in Brand.search(dom):
        loc = f'{SHOP_PATH}/brand/{env["ir.http"]._slugify(brand.name)}'
        if not qs or qs.lower() in loc:
            yield {'loc': loc}

    MachineType = env['machine.type']
    dom = sitemap_qs2dom(qs, f'{SHOP_PATH}/type', MachineType._rec_name)
    dom &= Domain('website_published', '=', True)
    for machine_type in MachineType.search(dom):
        loc = f'{SHOP_PATH}/type/{env["ir.http"]._slugify(machine_type.name)}'
        if not qs or qs.lower() in loc:
            yield {'loc': loc}

    if not qs or qs.lower() in f'{SHOP_PATH}/brands':
        yield {'loc': f'{SHOP_PATH}/brands'}
    if not qs or qs.lower() in f'{SHOP_PATH}/types':
        yield {'loc': f'{SHOP_PATH}/types'}


class WebsiteSaleMachineCatalog(WebsiteSale):

    def _parse_brand(self, brand):
        if not brand:
            brand = request.params.get('brand')

        if not brand:
            return False

        if isinstance(brand, int):
            return request.env['machine.brand'].browse(brand)

        if isinstance(brand, str) and brand.isdigit():
            return request.env['machine.brand'].browse(int(brand))

        if isinstance(brand, str):
            normalized_brand = request.env['ir.http']._slugify(brand.strip())
            for brand_record in request.env['machine.brand'].search([]):
                if request.env['ir.http']._slugify(brand_record.name) == normalized_brand:
                    return brand_record

        if hasattr(brand, 'exists'):
            return brand.exists()

        return False

    def _parse_machine_type(self, machine_type):
        if not machine_type:
            machine_type = request.params.get('machine_type')

        if not machine_type:
            return False

        if isinstance(machine_type, int):
            return request.env['machine.type'].browse(machine_type)

        if isinstance(machine_type, str) and machine_type.isdigit():
            return request.env['machine.type'].browse(int(machine_type))

        if isinstance(machine_type, str):
            normalized_machine_type = request.env['ir.http']._slugify(machine_type.strip())
            for machine_type_record in request.env['machine.type'].search([]):
                if request.env['ir.http']._slugify(machine_type_record.name) == normalized_machine_type:
                    return machine_type_record

        if hasattr(machine_type, 'exists'):
            return machine_type.exists()

        return False

    def _parse_machine_model(self, machine_model):
        if not machine_model:
            machine_model = request.params.get('machine_model_id')

        if not machine_model:
            return False

        if isinstance(machine_model, int):
            return request.env['machine.model'].browse(machine_model)

        if isinstance(machine_model, str) and machine_model.isdigit():
            return request.env['machine.model'].browse(int(machine_model))

        if isinstance(machine_model, str):
            normalized_machine_model = request.env['ir.http']._slugify(machine_model.strip())
            for machine_model_record in request.env['machine.model'].search([]):
                if request.env['ir.http']._slugify(machine_model_record.name) == normalized_machine_model:
                    return machine_model_record

        if hasattr(machine_model, 'exists'):
            return machine_model.exists()

        return False

    def _parse_vehicle_year(self, vehicle_year, machine_model):
        """Return the requested year as an int when it is one the vehicle model offers,
        otherwise 0 (the year is mandatory for vehicles, so 0 means "not selected")."""
        if not (machine_model and machine_model.is_vehicle):
            return 0
        if not vehicle_year:
            vehicle_year = request.params.get('vehicle_year')
        try:
            vehicle_year = int(vehicle_year)
        except (TypeError, ValueError):
            return 0
        return vehicle_year if vehicle_year in machine_model._get_available_years() else 0

    _MACHINE_FILTER_KEYS = ('page', 'category', 'brand', 'machine_type', 'machine_model_id', 'vehicle_year')

    def _machine_filter_url(self, category=None, brand=None, machine_type=None, machine_model=None, vehicle_year=None):
        """URL of the shop with the given machine filters and everything else of the
        current request (search, price, attributes, order...) preserved. Always built on
        /shop with query parameters, so it does not depend on which route mode
        (shop / brand / type / category) the customer is currently in."""
        args = request.httprequest.args.to_dict(flat=False)
        for key in self._MACHINE_FILTER_KEYS:
            args.pop(key, None)
        if category:
            args['category'] = [str(category.id)]
        if machine_type:
            args['machine_type'] = [str(machine_type.id)]
        if brand:
            args['brand'] = [str(brand.id)]
        if machine_model:
            args['machine_model_id'] = [str(machine_model.id)]
        if vehicle_year:
            args['vehicle_year'] = [str(vehicle_year)]
        query = urlencode(args, doseq=True)
        return f'{SHOP_PATH}?{query}' if query else SHOP_PATH

    def _get_machine_filters(self, category, brand, machine_type, machine_model, vehicle_year):
        """The shop's machine filters as a cascade of selects: Tipo → Marca → Modelo →
        Año (only for vehicles). Picking a value clears the ones below it, since their
        options depend on it. Each option carries the URL that applies it."""
        MachineType = request.env['machine.type']
        MachineBrand = request.env['machine.brand']
        published = [('website_published', '=', True)]
        url = lambda **kwargs: self._machine_filter_url(category, **kwargs)  # noqa: E731

        if machine_type:
            groups = request.env['machine.model']._read_group(
                published + [('machine_type_id', '=', machine_type.id), ('brand_id.website_published', '=', True)],
                groupby=['brand_id'],
            )
            brands = MachineBrand.union(*[record for record, in groups]).sorted(lambda record: (record.sequence, record.name))
        else:
            brands = MachineBrand.search(published)

        filters = [{
            'key': 'type',
            'label': 'Tipo',
            'placeholder': 'Todos los tipos',
            'all_url': url(),
            'options': [
                {'label': record.name, 'url': url(machine_type=record), 'active': record == machine_type}
                for record in MachineType.search(published)
            ],
            'active': bool(machine_type),
            'disabled': False,
            'required': False,
        }, {
            'key': 'brand',
            'label': 'Marca',
            'placeholder': 'Todas las marcas',
            'all_url': url(machine_type=machine_type),
            'options': [
                {'label': record.name, 'url': url(machine_type=machine_type, brand=record), 'active': record == brand}
                for record in brands
            ],
            'active': bool(brand),
            'disabled': False,
            'required': False,
        }]

        can_pick_model = bool(brand and machine_type)
        machine_models = self._get_shop_machine_models(brand, machine_type) if can_pick_model else request.env['machine.model']
        filters.append({
            'key': 'model',
            'label': 'Modelo',
            'placeholder': 'Todos los modelos' if can_pick_model else 'Selecciona tipo y marca',
            'all_url': url(machine_type=machine_type, brand=brand),
            'options': [
                {
                    'label': record.name,
                    'url': url(machine_type=machine_type, brand=brand, machine_model=record),
                    # A vehicle model is one row per year range: match by name.
                    'active': bool(machine_model) and (
                        record == machine_model or (record.is_vehicle and record.name == machine_model.name)
                    ),
                }
                for record in machine_models
            ],
            'active': bool(machine_model),
            'disabled': not can_pick_model,
            'required': False,
        })

        if machine_model and machine_model.is_vehicle:
            filters.append({
                'key': 'year',
                'label': 'Año',
                'placeholder': 'Selecciona el año',
                'all_url': url(machine_type=machine_type, brand=brand, machine_model=machine_model),
                'options': [
                    {
                        'label': str(year),
                        'url': url(machine_type=machine_type, brand=brand, machine_model=machine_model, vehicle_year=year),
                        'active': year == vehicle_year,
                    }
                    for year in machine_model._get_available_years()
                ],
                'active': bool(vehicle_year),
                'disabled': False,
                'required': True,
            })
        return filters

    def _get_shop_domain(self, search, category, attribute_value_dict, search_in_description=True, brand=None, machine_type=None, machine_model=None, vehicle_year=None):
        domain = super()._get_shop_domain(search, category, attribute_value_dict, search_in_description)
        brand = self._parse_brand(brand)
        if brand:
            domain = Domain.AND([domain, Domain('brand_ids', 'in', [brand.id])])
        machine_type = self._parse_machine_type(machine_type)
        if machine_type:
            domain = Domain.AND([domain, Domain('machine_type_ids', 'in', [machine_type.id])])
        machine_model = self._parse_machine_model(machine_model)
        if machine_model:
            fitment_ids = machine_model._fitment_ids_for_year(vehicle_year or request.params.get('vehicle_year'))
            # None: a vehicle model without a year is an incomplete selection, so it
            # does not filter yet (the shop asks for the year — see year_required).
            if fitment_ids is not None:
                domain = Domain.AND([domain, Domain('machine_model_ids', 'in', fitment_ids)])
        return domain

    def _get_shop_path(self, category=None, page=0, brand=None, machine_type=None):
        path = SHOP_PATH
        if brand:
            path += f'/brand/{request.env["ir.http"]._slugify(brand.name)}'
        elif machine_type:
            path += f'/type/{request.env["ir.http"]._slugify(machine_type.name)}'
        elif category:
            path += f'/category/{request.env["ir.http"]._slug(category)}'
        if page:
            path += f'/page/{page}'
        return path

    def _get_additional_shop_values(self, values, brand=None, **kwargs):
        values = super()._get_additional_shop_values(values, brand=brand, **kwargs)
        if brand:
            values['brand'] = brand
        return values

    def _get_shop_machine_models(self, brand, machine_type):
        """Models listed in the shop sidebar. A vehicle model is stored as one row per
        year range, so vehicles are listed once per name (the year is picked next)."""
        MachineModel = request.env['machine.model']
        if not (brand and machine_type):
            return MachineModel.browse()
        machine_models = MachineModel.search([
            ('website_published', '=', True),
            ('brand_id', '=', brand.id),
            ('machine_type_id', '=', machine_type.id),
        ])
        if not machine_type.is_vehicle:
            return machine_models
        first_by_name = {}
        for machine_model in machine_models:
            first_by_name.setdefault(machine_model.name, machine_model)
        return MachineModel.browse([record.id for record in first_by_name.values()])

    @http.route(
        [
            SHOP_PATH,
            f'{SHOP_PATH}/page/<int:page>',
            f'{SHOP_PATH}/category/<model("product.public.category"):category>',
            f'{SHOP_PATH}/category/<model("product.public.category"):category>/page/<int:page>',
            f'{SHOP_PATH}/brand/<string:brand>',
            f'{SHOP_PATH}/brand/<string:brand>/page/<int:page>',
            f'{SHOP_PATH}/type/<string:machine_type>',
            f'{SHOP_PATH}/type/<string:machine_type>/page/<int:page>',
        ],
        type='http',
        auth='public',
        website=True,
        list_as_website_content=_lt('Shop'),
        sitemap=sitemap_shop_with_catalogs,
        handle_params_access_error=lambda e, **kwargs: http.NotFound.code,
    )
    def shop(self, page=0, category=None, brand=None, machine_type=None, machine_model=None, vehicle_year=None, search='', min_price=0.0, max_price=0.0, tags='', **post):
        if not request.website.has_ecommerce_access():
            return request.redirect(f'/web/login?redirect={request.httprequest.path}')

        current_path = request.httprequest.path
        is_brand_route = current_path.startswith(f'{SHOP_PATH}/brand/')
        is_type_route = current_path.startswith(f'{SHOP_PATH}/type/')
        if is_brand_route:
            brand = self._parse_brand(brand)
            if not brand:
                return request.redirect(SHOP_PATH, code=302)
            if not brand.website_published:
                return http.NotFound()
            shop_route_mode = 'brand'
        elif is_type_route:
            machine_type = self._parse_machine_type(machine_type)
            if not machine_type:
                return request.redirect(SHOP_PATH, code=302)
            if not machine_type.website_published:
                return http.NotFound()
            shop_route_mode = 'type'
        elif current_path.startswith(f'{SHOP_PATH}/category/'):
            shop_route_mode = 'category'
        else:
            # When the search snippet submits to /shop with query parameters,
            # infer the active route mode from the selected machine filters so
            # active-filter chips can preserve the remaining filters correctly.
            if brand:
                shop_route_mode = 'brand'
            elif machine_type:
                shop_route_mode = 'type'
            elif category:
                shop_route_mode = 'category'
            else:
                shop_route_mode = 'shop'

        if not is_brand_route:
            brand = self._parse_brand(brand)
            if brand and not brand.website_published:
                return http.NotFound()
        if not is_type_route:
            machine_type = self._parse_machine_type(machine_type)
            if machine_type and not machine_type.website_published:
                return http.NotFound()

        machine_model = self._parse_machine_model(machine_model)
        if machine_model and not machine_model.website_published:
            return http.NotFound()

        category = self._validate_and_get_category(category or request.params.get('category'))

        try:
            min_price = float(min_price)
        except ValueError:
            min_price = 0
        try:
            max_price = float(max_price)
        except ValueError:
            max_price = 0

        website = request.env['website'].get_current_website()
        website_domain = website.website_domain()

        ppg = website.shop_ppg or 21
        ppr = website.shop_ppr or 4
        gap = website.shop_gap or '16px'

        request_args = request.httprequest.args
        attribute_values = request_args.getlist('attribute_values')
        attribute_value_dict = self._get_attribute_value_dict(attribute_values)
        attribute_ids = set(attribute_value_dict.keys())
        attribute_value_ids = set(itertools.chain.from_iterable(attribute_value_dict.values()))
        if attribute_values:
            request.session['attribute_values'] = attribute_values
            post['attribute_values'] = attribute_values
        else:
            request.session.pop('attribute_values', None)

        filter_by_tags_enabled = website.is_view_active('website_sale.filter_products_tags')
        if filter_by_tags_enabled:
            if tags:
                post['tags'] = tags
                tags = {request.env['ir.http']._unslug(tag)[1] for tag in tags.split(',')}
            else:
                post['tags'] = None
                tags = {}

        vehicle_year = self._parse_vehicle_year(vehicle_year, machine_model)
        year_required = bool(machine_model and machine_model.is_vehicle and not vehicle_year)
        if machine_model:
            post['machine_model_id'] = machine_model.id
        if vehicle_year:
            post['vehicle_year'] = vehicle_year

        if shop_route_mode == 'brand' and brand:
            url = f'{SHOP_PATH}/brand/{request.env["ir.http"]._slugify(brand.name)}'
        elif shop_route_mode == 'type' and machine_type:
            url = f'{SHOP_PATH}/type/{request.env["ir.http"]._slugify(machine_type.name)}'
        elif shop_route_mode == 'category' and category:
            url = f'{SHOP_PATH}/category/{request.env["ir.http"]._slug(category)}'
        else:
            url = SHOP_PATH

        query_url_kwargs = self._shop_get_query_url_kwargs(search, min_price, max_price, **post)
        if shop_route_mode == 'brand' and category:
            query_url_kwargs['category'] = category.id
        if shop_route_mode == 'brand' and machine_type:
            query_url_kwargs['machine_type'] = machine_type.id
        if shop_route_mode == 'type' and category:
            query_url_kwargs['category'] = category.id
        if shop_route_mode == 'type' and brand:
            query_url_kwargs['brand'] = brand.id
        if shop_route_mode == 'category' and brand:
            query_url_kwargs['brand'] = brand.id
        if shop_route_mode == 'category' and machine_type:
            query_url_kwargs['machine_type'] = machine_type.id
        if machine_model:
            query_url_kwargs['machine_model_id'] = machine_model.id
        if vehicle_year:
            query_url_kwargs['vehicle_year'] = vehicle_year
        keep = QueryURL(SHOP_PATH, **query_url_kwargs)

        # Check if we need to refresh the cached pricelist
        now = datetime.timestamp(datetime.now())
        if 'website_sale_pricelist_time' in request.session:
            pricelist_save_time = request.session['website_sale_pricelist_time']
            if pricelist_save_time < now - 60 * 60:
                request.session.pop('website_sale_pricelist_cache', None)
                request.session['website_sale_pricelist_time'] = now

        filter_by_price_enabled = website.is_view_active('website_sale.filter_products_price')
        if filter_by_price_enabled:
            company_currency = website.company_id.sudo().currency_id
            conversion_rate = request.env['res.currency']._get_conversion_rate(
                company_currency, website.currency_id, request.website.company_id, fields.Date.today())
        else:
            conversion_rate = 1

        if search:
            post['search'] = search

        options = self._get_search_options(
            category=category,
            attribute_value_dict=attribute_value_dict,
            min_price=min_price,
            max_price=max_price,
            conversion_rate=conversion_rate,
            display_currency=website.currency_id,
            **post,
        )
        if brand:
            options['brand_id'] = brand.id
        if machine_type:
            options['machine_type_id'] = machine_type.id
        if machine_model:
            options['machine_model_id'] = machine_model.id
        if vehicle_year:
            options['vehicle_year'] = vehicle_year
        fuzzy_search_term, product_count, search_product = self._shop_lookup_products(
            options, post, search, website
        )

        if filter_by_price_enabled:
            Product = request.env['product.template'].with_context(bin_size=True)
            search_term = fuzzy_search_term if fuzzy_search_term else search
            domain = self._get_shop_domain(
                search_term,
                category,
                attribute_value_dict,
                brand=brand,
                machine_type=machine_type,
                machine_model=machine_model,
                vehicle_year=vehicle_year,
            )

            query = Product._search(domain)
            sql = query.select(
                SQL(
                    'COALESCE(MIN(list_price), 0) * %(conversion_rate)s, COALESCE(MAX(list_price), 0) * %(conversion_rate)s',
                    conversion_rate=conversion_rate,
                )
            )
            available_min_price, available_max_price = request.env.execute_query(sql)[0]

            if min_price or max_price:
                if min_price:
                    min_price = min_price if min_price <= available_max_price else available_min_price
                    post['min_price'] = min_price
                if max_price:
                    max_price = max_price if max_price >= available_min_price else available_max_price
                    post['max_price'] = max_price

        ProductTag = request.env['product.tag']
        if filter_by_tags_enabled and search_product:
            all_tags = ProductTag.search_fetch(Domain.AND([
                Domain('visible_to_customers', '=', True),
                Domain.OR([
                    Domain('product_template_ids.is_published', '=', True),
                    Domain('product_ids.is_published', '=', True),
                ]),
                website_domain,
            ]))
        else:
            all_tags = ProductTag

        Category = request.env['product.public.category']
        categs_domain = Domain('parent_id', '=', False) & website_domain
        if not request.env.user._is_internal():
            categs_domain &= Domain('has_published_products', '=', True)
        if search:
            search_categories = Category.search(
                Domain('product_tmpl_ids', 'in', search_product.ids) & website_domain
            ).parents_and_self
            categs_domain &= Domain('id', 'in', search_categories.ids)
        else:
            search_categories = Category
        categs = Category.search_fetch(categs_domain)

        category_entries = Category
        if category:
            category_entries = not search and category.child_id or category.child_id.filtered(lambda c: c.id in search_categories.ids)
            if not category_entries:
                parent = category.parent_id
                category_entries = not search and parent.child_id or parent.child_id.filtered(lambda c: c.id in search_categories.ids)
        else:
            category_entries = categs
        if not request.env.user._is_internal():
            category_entries = category_entries.filtered('has_published_products')

        pager = website.pager(url=url, total=product_count, page=page, step=ppg, scope=5, url_args=post)
        offset = pager['offset']
        products = search_product[offset:offset + ppg]
        products.fetch()

        variants = request.env['product.product'].sudo().browse(product._get_first_possible_variant_id() for product in products)
        variants.fetch()
        product_variants = dict(zip(products, variants))

        ProductAttribute = request.env['product.attribute']
        attributes = ProductAttribute.browse(attribute_ids).sorted()
        if products:
            attributes_grouped = request.env['product.template.attribute.line']._read_group(
                domain=[
                    ('product_tmpl_id', 'in', search_product.ids),
                    ('attribute_id.visibility', '=', 'visible'),
                ],
                groupby=['attribute_id'],
                order='attribute_id'
            )
            attribute_ids = [attribute.id for attribute, in attributes_grouped]
            attributes = ProductAttribute.browse(attribute_ids).sorted()
        if website.is_view_active('website_sale.products_list_view'):
            layout_mode = 'list'
        else:
            layout_mode = 'grid'

        products_prices = products._get_sales_prices(website)
        product_query_params = self._get_product_query_params(**post)

        grouped_attributes_values = request.env['product.attribute.value'].browse(
            attribute_value_ids
        ).sorted().grouped('attribute_id')

        values = {
            'auto_assign_ribbons': self.env['product.ribbon'].sudo().search([('assign', '!=', 'manual')]),
            'search': fuzzy_search_term or search,
            'original_search': fuzzy_search_term and search,
            'order': post.get('order', ''),
            'brand': brand,
            'machine_type': machine_type,
            'machine_model': machine_model,
            'category': category,
            'attrib_values': attribute_value_dict,
            'attrib_set': attribute_value_ids,
            'pager': pager,
            'products': products,
            'product_variants': product_variants,
            'search_product': search_product,
            'search_count': product_count,
            'bins': TableCompute().process(products, ppg, ppr),
            'ppg': ppg,
            'ppr': ppr,
            'gap': gap,
            'categories': categs,
            'category_entries': category_entries,
            'attributes': attributes,
            'keep': keep,
            'shop_route_mode': shop_route_mode,
            'search_categories_ids': search_categories.ids,
            'layout_mode': layout_mode,
            'get_product_prices': lambda product: products_prices[product.id],
            'float_round': float_round,
            'shop_path': SHOP_PATH,
            'product_query_params': product_query_params,
            'grouped_attributes_values': grouped_attributes_values,
            'previewed_attribute_values': lazy(
                lambda: products._get_previewed_attribute_values(category, product_query_params),
            ),
            'brands': request.env['machine.brand'].search([('website_published', '=', True)]),
            'machine_types': request.env['machine.type'].search([('website_published', '=', True)]),
            'machine_models': self._get_shop_machine_models(brand, machine_type),
            'vehicle_year': vehicle_year,
            'year_required': year_required,
            'machine_filters': self._get_machine_filters(category, brand, machine_type, machine_model, vehicle_year),
        }
        if filter_by_price_enabled:
            values['min_price'] = min_price or available_min_price
            values['max_price'] = max_price or available_max_price
            values['available_min_price'] = float_round(available_min_price, 2)
            values['available_max_price'] = float_round(available_max_price, 2)
        if filter_by_tags_enabled:
            values.update({'all_tags': all_tags, 'tags': tags})
        if category:
            values['main_object'] = category
        elif brand:
            values['main_object'] = brand
        elif machine_type:
            values['main_object'] = machine_type
        elif machine_model:
            values['main_object'] = machine_model
        machine_selection_title = ''
        if machine_model and machine_type and brand:
            machine_selection_title = f'{machine_type.name} {brand.name} {machine_model.name}'
            if vehicle_year:
                machine_selection_title += f' {vehicle_year}'
        elif machine_type and brand:
            machine_selection_title = f'{machine_type.name} {brand.name}'
        elif machine_type:
            machine_selection_title = machine_type.name
        elif brand:
            machine_selection_title = brand.name
        values['machine_selection_title'] = machine_selection_title
        values.update(self._get_additional_shop_values(values, brand=brand, **post))
        machine_garage_record = None
        if brand and machine_type and machine_model and not request.env.user._is_public():
            machine_garage_record = request.env['machine.garage'].sudo().search([
                ('partner_id', '=', request.env.user.partner_id.commercial_partner_id.id),
                ('brand_id', '=', brand.id),
                ('machine_type_id', '=', machine_type.id),
                ('machine_model_id', '=', machine_model.id),
                ('vehicle_year', '=', vehicle_year),
            ], limit=1)
        values['machine_garage_record'] = machine_garage_record
        return request.render('website_sale.products', values)

    @http.route(
        [
            '/shop/brands',
        ],
        type='http',
        auth='public',
        website=True,
    )
    def brand_catalog(self, **kwargs):
        if not request.website.has_ecommerce_access():
            return request.redirect(f'/web/login?redirect={request.httprequest.path}')

        brands = request.env['machine.brand'].search([('website_published', '=', True)])
        return request.render('website_sale_machine_catalog.website_sale_brand_listing', {
            'brands': brands,
        })

    @http.route(
        [
            '/shop/types',
        ],
        type='http',
        auth='public',
        website=True,
    )
    def type_catalog(self, **kwargs):
        if not request.website.has_ecommerce_access():
            return request.redirect(f'/web/login?redirect={request.httprequest.path}')

        machine_types = request.env['machine.type'].search([('website_published', '=', True)])
        return request.render('website_sale_machine_catalog.website_sale_type_listing', {
            'machine_types': machine_types,
        })

    @http.route(
        ['/website_sale_machine_catalog/models'],
        type='json',
        auth='public',
        website=True,
        sitemap=False,
    )
    def machine_catalog_models(self, brand_id=None, machine_type_id=None, **kwargs):
        brand = self._parse_brand(brand_id)
        machine_type = self._parse_machine_type(machine_type_id)
        if not (brand and machine_type):
            return {'is_vehicle': False, 'models': []}
        machine_models = self._get_shop_machine_models(brand, machine_type).sorted(lambda record: (record.sequence, record.name))
        return {
            'is_vehicle': machine_type.is_vehicle,
            'models': [
                {'id': model.id, 'name': model.name, 'years': model._get_available_years()}
                for model in machine_models
            ],
        }

    @http.route(
        ['/website_sale_machine_catalog/search_types'],
        type='json',
        auth='public',
        website=True,
        sitemap=False,
    )
    def machine_catalog_search_types(self, **kwargs):
        """First step of the search snippet: what the catalog can be searched by.

        Only types that have published models are offered, so a type created empty
        (or left empty after its models were removed) never shows up as a dead end.
        Vehicles share a single type, flagged ``is_vehicle`` (see ADR 0006).
        """
        groups = request.env['machine.model']._read_group(
            [('website_published', '=', True), ('machine_type_id.website_published', '=', True)],
            groupby=['machine_type_id'],
        )
        types = request.env['machine.type'].union(*[machine_type for machine_type, in groups])
        types = types.sorted(lambda record: (record.sequence, record.name))
        vehicle_type = types.filtered('is_vehicle')[:1]
        return {
            'vehicle_type_id': vehicle_type.id or False,
            'machine_types': [{'id': record.id, 'name': record.name} for record in types if not record.is_vehicle],
        }

    @http.route(
        ['/website_sale_machine_catalog/search_brands'],
        type='json',
        auth='public',
        website=True,
        sitemap=False,
    )
    def machine_catalog_search_brands(self, machine_type_id=None, **kwargs):
        """Brands that have at least one published model of the given type."""
        machine_type = self._parse_machine_type(machine_type_id)
        if not machine_type:
            return []
        groups = request.env['machine.model']._read_group(
            [
                ('website_published', '=', True),
                ('machine_type_id', '=', machine_type.id),
                ('brand_id.website_published', '=', True),
            ],
            groupby=['brand_id'],
        )
        brands = request.env['machine.brand'].union(*[brand for brand, in groups])
        brands = brands.sorted(lambda record: (record.sequence, record.name))
        return [{'id': brand.id, 'name': brand.name} for brand in brands]

    @http.route(
        ['/my/garage'],
        type='http',
        auth='user',
        website=True,
        sitemap=False,
    )
    def machine_garage(self, **kwargs):
        partner = request.env.user.partner_id.commercial_partner_id
        garage_records = request.env['machine.garage'].sudo().search([
            ('partner_id', '=', partner.id),
        ], order='create_date desc, id desc')
        return request.render('website_sale_machine_catalog.machine_garage_page', {
            'garage_records': garage_records,
        })

    @http.route(
        ['/my/garage/add'],
        type='http',
        auth='user',
        website=True,
        methods=['POST'],
    )
    def machine_garage_add(self, brand_id=None, machine_type_id=None, machine_model_id=None, vehicle_year=None, **post):
        brand = self._parse_brand(brand_id)
        machine_type = self._parse_machine_type(machine_type_id)
        machine_model = self._parse_machine_model(machine_model_id)
        if not (brand and machine_type and machine_model):
            return request.redirect('/my/garage')
        if machine_model.brand_id != brand or machine_model.machine_type_id != machine_type:
            return request.redirect('/my/garage')
        vehicle_year = self._parse_vehicle_year(vehicle_year, machine_model)
        if machine_type.is_vehicle and not vehicle_year:
            return request.redirect('/my/garage')

        partner = request.env.user.partner_id.commercial_partner_id
        Garage = request.env['machine.garage'].sudo()
        garage = Garage.search([
            ('partner_id', '=', partner.id),
            ('brand_id', '=', brand.id),
            ('machine_type_id', '=', machine_type.id),
            ('machine_model_id', '=', machine_model.id),
            ('vehicle_year', '=', vehicle_year),
        ], limit=1)
        if not garage:
            Garage.create({
                'partner_id': partner.id,
                'brand_id': brand.id,
                'machine_type_id': machine_type.id,
                'machine_model_id': machine_model.id,
                'vehicle_year': vehicle_year,
            })
        return request.redirect('/my/garage')

    @http.route(
        ['/my/garage/<int:garage_id>/remove'],
        type='http',
        auth='user',
        website=True,
        methods=['POST'],
    )
    def machine_garage_remove(self, garage_id, **post):
        partner = request.env.user.partner_id.commercial_partner_id
        garage = request.env['machine.garage'].sudo().search([
            ('id', '=', garage_id),
            ('partner_id', '=', partner.id),
        ], limit=1)
        if garage:
            garage.unlink()
        return request.redirect('/my/garage')
