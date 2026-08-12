El proyecto está compuesto por tres servicios independientes.

admin-dashboard

Responsable del frontend.

Nunca accede directamente al ERP.

Siempre consume el Core Orchestrator.

-----------------------------------

synapse-bridge

Única aplicación autorizada para conectarse a Azure Synapse.

No modifica datos.

Lee información.

Publica eventos.

Actualiza Redis.

-----------------------------------

core-orchestrator

Backend principal.

Responsable de:

- API REST
- lógica de negocio
- Marketplace
- Odoo
- Amazon
- MercadoLibre
- atributos adicionales
- imágenes
- stock
- precios

Nunca consulta directamente Azure Synapse.

Obtiene la información desde Redis y RabbitMQ.