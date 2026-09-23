# Part of Odoo. See LICENSE file for full copyright and licensing details.

OLD_MODULE = 'website_sale_machine_catalog'
NEW_MODULE = 'website_vegusa'


def migrate(cr, version):
    """website_sale_machine_catalog has been folded into website_vegusa.

    The machine.* models, their fields, views, menus and access rules are already in the
    database owned by the old module. Uninstalling it would drop them (and their data), so
    instead of installing/uninstalling, ownership is transferred: the records keep their
    external identifier name and only change module, so when website_vegusa loads its data
    files it updates them in place. Safe to run when the old module was never installed.
    """
    # 1.1.0 of the old module: the year became part of a model / garage entry's identity,
    # so the previous unique constraints must go before the models are updated.
    cr.execute("ALTER TABLE IF EXISTS machine_model DROP CONSTRAINT IF EXISTS machine_model_machine_model_brand_type_name_unique")
    cr.execute("ALTER TABLE IF EXISTS machine_garage DROP CONSTRAINT IF EXISTS machine_garage_machine_garage_partner_combo_unique")

    cr.execute("SELECT id FROM ir_module_module WHERE name = %s", (OLD_MODULE,))
    old = cr.fetchone()
    if not old:
        return
    old_id = old[0]
    cr.execute("SELECT id FROM ir_module_module WHERE name = %s", (NEW_MODULE,))
    new_id = cr.fetchone()[0]

    # External identifiers (models, fields, views, actions, menus, access rules...).
    cr.execute(
        """
        DELETE FROM ir_model_data old_data
              USING ir_model_data new_data
              WHERE old_data.module = %s AND new_data.module = %s AND old_data.name = new_data.name
        """,
        (OLD_MODULE, NEW_MODULE),
    )
    cr.execute("UPDATE ir_model_data SET module = %s WHERE module = %s", (NEW_MODULE, OLD_MODULE))

    # Constraints and many2many tables that Odoo removes when their module is uninstalled.
    cr.execute("UPDATE ir_model_constraint SET module = %s WHERE module = %s", (new_id, old_id))
    cr.execute("UPDATE ir_model_relation SET module = %s WHERE module = %s", (new_id, old_id))

    # Website views keep their xmlid in "key": t-call/inherit lookups go through it.
    cr.execute(
        "UPDATE ir_ui_view SET key = %s || substr(key, %s) WHERE key LIKE %s",
        (NEW_MODULE + '.', len(OLD_MODULE) + 2, OLD_MODULE + '.%'),
    )

    # The old module no longer exists: mark it uninstalled without running its uninstall hooks.
    cr.execute("DELETE FROM ir_module_module_dependency WHERE module_id = %s", (old_id,))
    cr.execute("UPDATE ir_module_module SET state = 'uninstalled', latest_version = NULL WHERE id = %s", (old_id,))
