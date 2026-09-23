# Part of Odoo. See LICENSE file for full copyright and licensing details.

from odoo import api, fields, models
from odoo.exceptions import ValidationError


class ProductType(models.Model):
    _name = 'machine.type'
    _description = 'Machine Type'
    _order = 'sequence, name'
    _sql_constraints = [
        ('machine_type_name_unique', 'unique(name)', 'The machine type name must be unique.'),
    ]
    _ecom_ref_unique = models.Constraint('UNIQUE(ecom_ref)', 'The ecom reference must be unique.')

    name = fields.Char(required=True, index=True)
    is_vehicle = fields.Boolean(
        string='Is Vehicle',
        help='Models of this type are vehicles: they carry a year range and the '
             'website requires the customer to pick a year when filtering by them.',
    )
    ecom_ref = fields.Char(
        string='Ecom Reference',
        index=True,
        copy=False,
        readonly=True,
        help='Identifier of this record in core-orchestrator (set by the sync). '
             'Empty for records created by hand in Odoo and for the vehicle type.',
    )
    sequence = fields.Integer(default=10)
    active = fields.Boolean(default=True)
    website_published = fields.Boolean(default=True)
    logo = fields.Image(string='Logo')
    description = fields.Text(string='Description')
    product_tmpl_ids = fields.Many2many(
        'product.template',
        relation='machine_type_product_template_rel',
        column1='type_id',
        column2='product_tmpl_id',
        string='Products',
        readonly=True,
    )
    product_count = fields.Integer(
        string='Product Count', compute='_compute_product_count', readonly=True
    )

    @api.depends('product_tmpl_ids')
    def _compute_product_count(self):
        for machine_type in self:
            machine_type.product_count = len(machine_type.product_tmpl_ids)

    @api.constrains('name')
    def _check_unique_name(self):
        for machine_type in self.filtered('name'):
            duplicate = self.search_count([
                ('id', '!=', machine_type.id),
                ('name', '=ilike', machine_type.name.strip()),
            ])
            if duplicate:
                raise ValidationError('The machine type name must be unique.')

    @api.model
    def _ecom_upsert(self, data):
        """Find or create the type described by ``data`` ({'ecom_ref', 'name', 'is_vehicle'}).

        Same matching rules as machine.brand: by ``ecom_ref``, else adopt a hand-created
        type with the same name. The vehicle type has no ``ecom_ref`` (vehicles have no
        type table in core-orchestrator) and is matched by name.
        """
        ref = data.get('ecom_ref') or False
        name = (data.get('name') or '').strip()
        is_vehicle = bool(data.get('is_vehicle'))
        machine_type = self.with_context(active_test=False).search([('ecom_ref', '=', ref)], limit=1) if ref else self.browse()
        if not machine_type and name:
            machine_type = self.with_context(active_test=False).search([('name', '=ilike', name)]).filtered(
                lambda record: not record.ecom_ref
            )[:1]
        if machine_type:
            vals = {}
            if ref and machine_type.ecom_ref != ref:
                vals['ecom_ref'] = ref
            if machine_type.is_vehicle != is_vehicle:
                vals['is_vehicle'] = is_vehicle
            if name and machine_type.name != name and not self.search_count([('id', '!=', machine_type.id), ('name', '=ilike', name)]):
                vals['name'] = name
            if vals:
                machine_type.write(vals)
            return machine_type
        return self.create({'name': name, 'ecom_ref': ref, 'is_vehicle': is_vehicle})
