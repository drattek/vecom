Todas las bases de datos utilizan el sistema de Soft Delete

Siempre se deben marcar con el atributo deleted_at

--------------------------------

Los precios siempre se pueden almacenar en USD y MXN.

Se utiliza la relación con currency según aplique.

--------------------------------

Todos los marketplaces reciben MXN.

--------------------------------

El ERP nunca almacena atributos personalizados.

--------------------------------

Los atributos personalizados pertenecen exclusivamente al Core.

--------------------------------

Redis es cache.

Nunca es la fuente de verdad.

--------------------------------

Azure Synapse es solo lectura.

--------------------------------

Synapse Bridge puede consultar de diversos ERP

Existe conexión hacia ERP para maquinaria.

Existe conexión hacia ERP de Vehículos Nissan.

-------------------------------