# Part of Odoo. See LICENSE file for full copyright and licensing details.


def migrate(cr, version):
    """1.1.0 makes the year part of a machine.model / machine.garage identity, so the
    old unique constraints (which ignore it) must go before the models are updated."""
    cr.execute("ALTER TABLE machine_model DROP CONSTRAINT IF EXISTS machine_model_machine_model_brand_type_name_unique")
    cr.execute("ALTER TABLE machine_garage DROP CONSTRAINT IF EXISTS machine_garage_machine_garage_partner_combo_unique")
