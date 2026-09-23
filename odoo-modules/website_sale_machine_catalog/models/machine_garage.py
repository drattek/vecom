# Part of Odoo. See LICENSE file for full copyright and licensing details.

from odoo import api, fields, models
from odoo.exceptions import ValidationError


class MachineGarage(models.Model):
    _name = 'machine.garage'
    _description = 'Machine Garage'
    _rec_name = 'name'
    _order = 'create_date desc, id desc'

    name = fields.Char(compute='_compute_name', store=True, readonly=True)
    partner_id = fields.Many2one('res.partner', required=True, index=True, ondelete='cascade')
    brand_id = fields.Many2one('machine.brand', required=True, index=True, ondelete='restrict')
    machine_type_id = fields.Many2one('machine.type', required=True, index=True, ondelete='restrict')
    machine_model_id = fields.Many2one('machine.model', required=True, index=True, ondelete='restrict')
    # Mandatory when the machine type is a vehicle; 0 for machinery.
    vehicle_year = fields.Integer(string='Year')

    _partner_combo_year_unique = models.Constraint(
        'UNIQUE(partner_id, brand_id, machine_type_id, machine_model_id, vehicle_year)',
        'This machine combination is already saved in the garage.',
    )

    @api.constrains('machine_type_id', 'vehicle_year')
    def _check_vehicle_year(self):
        for garage in self:
            if garage.machine_type_id.is_vehicle and not garage.vehicle_year:
                raise ValidationError('The year is required for vehicles.')

    @api.depends('brand_id.name', 'machine_type_id.name', 'machine_model_id.name', 'vehicle_year')
    def _compute_name(self):
        for garage in self:
            garage.name = ' / '.join([
                garage.brand_id.name or '',
                garage.machine_type_id.name or '',
                garage.machine_model_id.name or '',
                str(garage.vehicle_year) if garage.vehicle_year else '',
            ]).strip(' /')

    def get_shop_url(self):
        self.ensure_one()
        url = '/shop/brand/%s?machine_type=%s&machine_model_id=%s' % (
            self.env['ir.http']._slugify(self.brand_id.name),
            self.machine_type_id.id,
            self.machine_model_id.id,
        )
        if self.vehicle_year:
            url += '&vehicle_year=%s' % self.vehicle_year
        return url