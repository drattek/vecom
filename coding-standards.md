# Coding Standards

## Go

- Usar `context.Context` en todos los métodos que accedan a I/O, servicios externos, bases de datos o flujos de request.
- Evitar variables globales y estado compartido mutable a nivel paquete.
- Toda función o método público debe tener comentario si su propósito no es obvio.
- Devolver errores envueltos con `fmt.Errorf("...: %w", err)` para preservar el contexto de falla.
- Organizar el código siguiendo la arquitectura del proyecto:
  - `cmd/` para entrypoints.
  - `internal/api/` para routers y wiring HTTP.
  - `internal/application/` para casos de uso y lógica de negocio.
  - `internal/domain/` para modelos y reglas del dominio.
  - `internal/infrastructure/` para implementación de persistencia, brokers, clientes externos.
  - `internal/interfaces/` para handlers y contratos de entrada/salida.
- Mantener los handlers delgados: no poner lógica de negocio directamente en el controlador.
- Favorer la inyección de dependencias por constructor y evitar acoplamiento fuerte a implementaciones concretas.
- Usar nombres claros para paquetes y evitar funciones demasiado largas.
- Para operaciones de base de datos, mantener separación entre consulta, transformación y validación.

## Java

- Usar Spring Boot siguiendo una arquitectura limpia y modular.
- Preferir Constructor Injection sobre Field Injection.
- Separar responsabilidades en capas: controller, service, repository, mapper/model.
- Evitar lógica de negocio en controladores; dejarla en servicios.
- Usar DTOs para separar la capa HTTP del dominio.
- Aplicar validación de entradas con Bean Validation (`@Valid`, `@NotNull`, etc.).
- Mantener los servicios pequeños y orientados a casos de uso concretos.
- Preferir interfaces para contratos y facilitar pruebas y sustitución de dependencias.
- Manejar excepciones de forma explícita y devolver respuestas consistentes.
- Usar nombres de clases y métodos descriptivos y seguir convenciones Spring (`@RestController`, `@Service`, `@Repository`).

## React

- Usar React con Hooks y componentes funcionales; evitar clases.
- Organizar por Feature Folder, agrupando componentes, hooks, servicios y tipos del mismo dominio.
- Usar TypeScript estrictamente y definir tipos claros para props, responses y estado.
- Usar TanStack Query para manejar estado del servidor y peticiones HTTP.
- Usar React Hook Form junto con Zod para formularios y validación de entradas.
- Mantener los componentes pequeños, reutilizables y con una responsabilidad única.
- Usar Tailwind CSS para estilado y Shadcn/ui para componentes base cuando sea apropiado.
- Favorer Zustand para estado local simple y compartido cuando no sea necesario un store complejo.
- Separar lógica de negocio y efectos de UI; los hooks personalizados deben encapsular comportamientos reutilizables.
- Mantener los endpoints de API centralizados en una capa de servicio o cliente HTTP para facilitar cambios futuros.
- Respetar patrones de acceso protegido y manejo de errores en la interfaz.

## Reglas generales para generación de código

- Generar código que se integre con la arquitectura existente en lugar de introducir patrones paralelos.
- Priorizar claridad, mantenibilidad y consistencia sobre cleverness.
- Cuando exista una capa de dominio o aplicación, no colocar lógica de negocio directamente en el transporte o en la UI.
- Al crear nuevas entidades o módulos, revisar si ya existe un patrón similar en el proyecto antes de introducir uno nuevo.
- Mantener compatibilidad con los estándares del repositorio: nombres en inglés cuando aplique, estructura modular y separación de responsabilidades.