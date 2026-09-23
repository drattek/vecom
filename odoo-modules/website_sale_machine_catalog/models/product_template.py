# Part of Odoo. See LICENSE file for full copyright and licensing details.

from odoo import api, fields, models
from odoo.fields import Domain


class ProductTemplate(models.Model):
    _inherit = 'product.template'

    brand_ids = fields.Many2many(
        'machine.brand',
        relation='machine_brand_product_template_rel',
        column1='product_tmpl_id',
        column2='brand_id',
        string='Brands',
    )

    machine_type_ids = fields.Many2many(
        'machine.type',
        relation='machine_type_product_template_rel',
        column1='product_tmpl_id',
        column2='type_id',
        string='Types',
    )

    machine_model_ids = fields.Many2many(
        'machine.model',
        relation='machine_model_product_template_rel',
        column1='product_tmpl_id',
        column2='model_id',
        string='Models',
    )

    @api.model
    def _search_get_detail(self, website, order, options):
        detail = super()._search_get_detail(website, order, options)
        brand_id = options.get('brand_id')
        if brand_id:
            detail['base_domain'].append(Domain('brand_ids', 'in', [brand_id]))
        machine_type_id = options.get('machine_type_id')
        if machine_type_id:
            detail['base_domain'].append(Domain('machine_type_ids', 'in', [machine_type_id]))
        machine_model_id = options.get('machine_model_id')
        if machine_model_id:
            fitment_ids = self.env['machine.model'].browse(machine_model_id)._fitment_ids_for_year(
                options.get('vehicle_year')
            )
            # None: a vehicle model without a year is an incomplete selection.
            if fitment_ids is not None:
                detail['base_domain'].append(Domain('machine_model_ids', 'in', fitment_ids))
        return detail

    def ecom_sync_fitments(self, fitments):
        """Entry point for core-orchestrator (Odoo JSON-2 API, called with the template id).

        ``fitments`` is the complete list of machine/vehicle compatibilities the product
        has in core-orchestrator; each item is
        {'ecom_ref', 'name', 'year_start', 'year_end',
         'brand': {'ecom_ref', 'name'}, 'type': {'ecom_ref', 'name', 'is_vehicle'}}.

        Models are upserted (see machine.model._ecom_upsert) and the product's managed
        models — the ones carrying an ``ecom_ref`` — are replaced by that list. Models
        without ``ecom_ref`` (loaded by hand) are never removed. Brands and machine
        types are kept consistent with the models: the ones backed only by a removed
        model are dropped and the ones backed by the final models are added, leaving any
        other hand-assigned brand or type alone. An empty list therefore only removes
        what a previous sync had assigned. Returns a summary dict.
        """
        self.ensure_one()
        Model = self.env['machine.model']
        synced = Model.browse()
        for fitment in fitments or []:
            synced |= Model._ecom_upsert(fitment)

        current = self.machine_model_ids
        removed = current.filtered('ecom_ref') - synced
        final = (current - removed) | synced
        if final == current:
            return {'linked': 0, 'unlinked': 0}

        self.write({
            'machine_model_ids': [(6, 0, final.ids)],
            'brand_ids': [(6, 0, ((self.brand_ids - removed.brand_id) | final.brand_id).ids)],
            'machine_type_ids': [(6, 0, ((self.machine_type_ids - removed.machine_type_id) | final.machine_type_id).ids)],
        })
        return {'linked': len(final - current), 'unlinked': len(current - final)}
