Proyecto: Middleware Ecommerce

Objetivo

Sincronizar información proveniente de Azure Synapse hacia un sistema propio para posteriormente distribuir la información a diferentes marketplaces y plataformas.

Tecnologías

- Go
- Java
- React
- Redis
- RabbitMQ
- MariaDB
- Docker

Arquitectura

ERP
    ↓
Synapse Bridge
    ↓
Redis
RabbitMQ
    ↓
Core Orchestrator
    ↓
Marketplace APIs