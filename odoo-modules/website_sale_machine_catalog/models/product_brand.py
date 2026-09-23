# Part of Odoo. See LICENSE file for full copyright and licensing details.

from odoo import api, fields, models
from odoo.exceptions import ValidationError


class ProductBrand(models.Model):
    _name = 'machine.brand'
    _description = 'Machine Brand'
    _order = 'sequence, name'
    _sql_constraints = [
        ('machine_brand_name_unique', 'unique(name)', 'The brand name must be unique.'),
    ]
    _ecom_ref_unique = models.Constraint('UNIQUE(ecom_ref)', 'The ecom reference must be unique.')

    name = fields.Char(required=True, index=True)
    ecom_ref = fields.Char(
        string='Ecom Reference',
        index=True,
        copy=False,
        readonly=True,
        help='Identifier of this record in core-orchestrator (set by the sync). '
             'Empty for records created by hand in Odoo.',
    )
    sequence = fields.Integer(default=10)
    active = fields.Boolean(default=True)
    website_published = fields.Boolean(default=True)
    logo = fields.Image(string='Logo')
    description = fields.Text(string='Description')
    product_tmpl_ids = fields.Many2many(
        'product.template',
        relation='machine_brand_product_template_rel',
        column1='brand_id',
        column2='product_tmpl_id',
        string='Products',
        readonly=True,
    )
    product_count = fields.Integer(
        string='Product Count', compute='_compute_product_count', readonly=True
    )

    @api.depends('product_tmpl_ids')
    def _compute_product_count(self):
        for brand in self:
            brand.product_count = len(brand.product_tmpl_ids)

    @api.constrains('name')
    def _check_unique_name(self):
        for brand in self.filtered('name'):
            duplicate = self.search_count([
                ('id', '!=', brand.id),
                ('name', '=ilike', brand.name.strip()),
            ])
            if duplicate:
                raise ValidationError('The brand name must be unique.')

    @api.model
    def _ecom_upsert(self, data):
        """Find or create the brand described by ``data`` ({'ecom_ref', 'name'}).

        Matches first by ``ecom_ref``; otherwise adopts a hand-created brand with the
        same name (stamping its ``ecom_ref``) so the initial sync does not duplicate
        the manually loaded catalog.
        """
        ref = data.get('ecom_ref') or False
        name = (data.get('name') or '').strip()
        brand = self.with_context(active_test=False).search([('ecom_ref', '=', ref)], limit=1) if ref else self.browse()
        if not brand and name:
            brand = self.with_context(active_test=False).search([('name', '=ilike', name)]).filtered(
                lambda record: not record.ecom_ref
            )[:1]
        if brand:
            vals = {}
            if ref and brand.ecom_ref != ref:
                vals['ecom_ref'] = ref
            if name and brand.name != name and not self.search_count([('id', '!=', brand.id), ('name', '=ilike', name)]):
                vals['name'] = name
            if vals:
                brand.write(vals)
            return brand
        return self.create({'name': name, 'ecom_ref': ref})
