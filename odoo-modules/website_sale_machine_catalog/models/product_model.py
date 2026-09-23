# Part of Odoo. See LICENSE file for full copyright and licensing details.

from datetime import date

from odoo import api, fields, models
from odoo.exceptions import ValidationError


class ProductModel(models.Model):
    _name = 'machine.model'
    _description = 'Machine Model'
    _order = 'sequence, brand_id, machine_type_id, name, year_start'
    _ecom_ref_unique = models.Constraint('UNIQUE(ecom_ref)', 'The ecom reference must be unique.')

    name = fields.Char(required=True, index=True)
    brand_id = fields.Many2one('machine.brand', required=True, ondelete='restrict', string='Brand')
    machine_type_id = fields.Many2one(
        'machine.type', required=True, ondelete='restrict', string='Type'
    )
    is_vehicle = fields.Boolean(related='machine_type_id.is_vehicle', string='Is Vehicle')
    # 0 means "not set": machinery has no years, and year_end == 0 on a vehicle means
    # "from year_start onwards" (mirrors ecom_vehicle_fitments.year_end NULL).
    year_start = fields.Integer(string='Year From')
    year_end = fields.Integer(string='Year To')
    year_label = fields.Char(compute='_compute_year_label', string='Years')
    ecom_ref = fields.Char(
        string='Ecom Reference',
        index=True,
        copy=False,
        readonly=True,
        help='Identifier of this fitment in core-orchestrator (set by the sync). '
             'Empty for records created by hand in Odoo.',
    )
    sequence = fields.Integer(default=10)
    active = fields.Boolean(default=True)
    website_published = fields.Boolean(default=True)
    logo = fields.Image(string='Logo')
    description = fields.Text(string='Description')
    product_tmpl_ids = fields.Many2many(
        'product.template',
        relation='machine_model_product_template_rel',
        column1='model_id',
        column2='product_tmpl_id',
        string='Products',
        readonly=True,
    )
    product_count = fields.Integer(
        string='Product Count', compute='_compute_product_count', readonly=True
    )

    @api.depends('product_tmpl_ids')
    def _compute_product_count(self):
        for machine_model in self:
            machine_model.product_count = len(machine_model.product_tmpl_ids)

    @api.depends('year_start', 'year_end')
    def _compute_year_label(self):
        for machine_model in self:
            if machine_model.year_start and machine_model.year_end and machine_model.year_end != machine_model.year_start:
                machine_model.year_label = '%s-%s' % (machine_model.year_start, machine_model.year_end)
            elif machine_model.year_start and machine_model.year_end:
                machine_model.year_label = str(machine_model.year_start)
            elif machine_model.year_start:
                machine_model.year_label = '%s+' % machine_model.year_start
            else:
                machine_model.year_label = False

    @api.depends('name', 'year_label')
    def _compute_display_name(self):
        # The same vehicle name exists once per year range, so backend widgets
        # (many2many tags on the product form) need the years to tell them apart.
        for machine_model in self:
            machine_model.display_name = (
                '%s (%s)' % (machine_model.name, machine_model.year_label)
                if machine_model.year_label else machine_model.name
            )

    @api.constrains('name', 'brand_id', 'machine_type_id', 'year_start', 'year_end')
    def _check_unique_name(self):
        for machine_model in self.filtered(lambda record: record.name and record.brand_id and record.machine_type_id):
            duplicate = self.search_count([
                ('id', '!=', machine_model.id),
                ('brand_id', '=', machine_model.brand_id.id),
                ('machine_type_id', '=', machine_model.machine_type_id.id),
                ('name', '=ilike', machine_model.name.strip()),
                ('year_start', '=', machine_model.year_start),
                ('year_end', '=', machine_model.year_end),
            ])
            if duplicate:
                raise ValidationError(
                    'The machine model name must be unique for the selected brand, machine type and years.'
                )

    @api.constrains('machine_type_id', 'year_start', 'year_end')
    def _check_years(self):
        for machine_model in self:
            if machine_model.year_end and machine_model.year_end < machine_model.year_start:
                raise ValidationError('The end year cannot be before the start year.')
            if machine_model.machine_type_id.is_vehicle and not machine_model.year_start:
                raise ValidationError('Vehicle models require a start year.')

    # ------------------------------------------------------------------
    # Year filtering (vehicles)
    # ------------------------------------------------------------------

    def _year_domain(self, year):
        return [
            ('year_start', '<=', year),
            '|', ('year_end', '=', 0), ('year_end', '>=', year),
        ]

    def _get_same_name_models(self):
        """Every published row of the same brand/type/name — for vehicles, one model
        name is split in several rows, one per year range."""
        self.ensure_one()
        return self.search([
            ('website_published', '=', True),
            ('brand_id', '=', self.brand_id.id),
            ('machine_type_id', '=', self.machine_type_id.id),
            ('name', '=', self.name),
        ])

    def _get_available_years(self):
        """Years (descending) a customer can pick for this vehicle model: the union of
        the year ranges of every row sharing its brand/type/name. Open-ended ranges
        run up to next year."""
        self.ensure_one()
        if not self.is_vehicle:
            return []
        last_year = date.today().year + 1
        years = set()
        for machine_model in self._get_same_name_models():
            if not machine_model.year_start:
                continue
            years.update(range(machine_model.year_start, (machine_model.year_end or last_year) + 1))
        return sorted(years, reverse=True)

    def _fitment_ids_for_year(self, year=None):
        """Ids of the machine.model rows a product must be linked to in order to match
        this model in the shop, or None when the selection is incomplete.

        Machinery matches the exact row. Vehicles match every row with the same
        brand/type/name whose year range covers ``year``; a vehicle without a valid
        year is incomplete (the year is mandatory), so it returns None and callers
        skip the model filter.
        """
        self.ensure_one()
        if not self.is_vehicle:
            return self.ids
        try:
            year = int(year)
        except (TypeError, ValueError):
            return None
        if year <= 0:
            return None
        return self.search([
            ('brand_id', '=', self.brand_id.id),
            ('machine_type_id', '=', self.machine_type_id.id),
            ('name', '=', self.name),
        ] + self._year_domain(year)).ids

    # ------------------------------------------------------------------
    # core-orchestrator sync
    # ------------------------------------------------------------------

    @api.model
    def _ecom_upsert(self, data):
        """Find or create the fitment described by ``data``:
        {'ecom_ref', 'name', 'brand': {...}, 'type': {...}, 'year_start', 'year_end'}.

        Matches by ``ecom_ref``; otherwise adopts a hand-created row with the same
        brand/type/name/years (stamping its ``ecom_ref``) so the first sync does not
        duplicate the manually loaded catalog.
        """
        brand = self.env['machine.brand']._ecom_upsert(data['brand'])
        machine_type = self.env['machine.type']._ecom_upsert(data['type'])
        ref = data.get('ecom_ref') or False
        name = (data.get('name') or '').strip()
        year_start = int(data.get('year_start') or 0)
        year_end = int(data.get('year_end') or 0)
        values = {
            'name': name,
            'brand_id': brand.id,
            'machine_type_id': machine_type.id,
            'year_start': year_start,
            'year_end': year_end,
        }

        Model = self.with_context(active_test=False)
        machine_model = Model.search([('ecom_ref', '=', ref)], limit=1) if ref else self.browse()
        if not machine_model:
            machine_model = Model.search([
                ('brand_id', '=', brand.id),
                ('machine_type_id', '=', machine_type.id),
                ('name', '=ilike', name),
                ('year_start', '=', year_start),
                ('year_end', '=', year_end),
            ]).filtered(lambda record: not record.ecom_ref)[:1]
        if machine_model:
            changed = {
                field: value for field, value in values.items()
                if machine_model[field] != value and not (field in ('brand_id', 'machine_type_id') and machine_model[field].id == value)
            }
            if ref and machine_model.ecom_ref != ref:
                changed['ecom_ref'] = ref
            if changed:
                machine_model.write(changed)
            return machine_model
        values['ecom_ref'] = ref
        return self.create(values)
