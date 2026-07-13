# AstreoGateway — Deploy Sprints
# Alcance: deploy usable (P0 + ops básica)
# Cada sprint cabe en 1 sesión IA. Ejecutar en orden; no saltar P0.

## S01 | WriteTimeout → 0
goal: streams largos no se cortan a los 60s por el HTTP server
scope: cmd/aigw/main.go
steps:
  - Cambiar WriteTimeout de 60s a 0 (o configurable >= proxy timeout)
  - Si se expone flag, registrarlo en config.Load
done:
  - WriteTimeout = 0 en http.Server
  - go vet ./... && go test ./... pasan
verify: go test ./...
deps: none
status: done

## S02 | MkdirAll path DB
goal: arranque Docker sin directorio preexistente no falla
scope: internal/store/store.go
steps:
  - Al inicio de Open(), os.MkdirAll(filepath.Dir(dbPath), 0o755) solo si dbPath contiene directorio
  - Importar "os" y "path/filepath"
done:
  - `-db /tmp/aigw-test/new-dir/db.sqlite` crea el árbol y abre DB sin error
  - go vet ./... && go test ./... pasan
verify: go test ./...
deps: none
status: done

## S03 | docker-compose + VOLUME
goal: deploy de un solo comando con volume persistente
scope: docker-compose.yml, Dockerfile (VOLUME), README sección Docker
steps:
  - Crear docker-compose.yml con servicio aigw, volume, restart: unless-stopped, ports, env de ejemplo
  - Añadir VOLUME /app/data en Dockerfile
  - Actualizar README sección Docker: docker compose up --build -d, healthcheck externo
done:
  - docker compose up --build -d && curl localhost:18473/healthz devuelve 200
  - reinicio del contenedor preserva datos
verify: docker compose up --build -d && curl -s localhost:18473/healthz && docker compose down
deps: S02
status: done

## S04 | Runbook deploy
goal: documentación completa para deploy en producción
scope: docs/deploy.md (nuevo), + link en README
steps:
  - Crear docs/deploy.md con:
    - Bootstrap paso a paso
    - Reverse proxy (Caddy/nginx) + AIGW_COOKIE_SECURE=true
    - Permisos volume nonroot
    - Backup/restore SQLite
    - Upgrade de imagen
    - Checklist mínimo
  - Añadir línea en README apuntando a docs/deploy.md
done:
  - docs/deploy.md existe con todas las secciones
  - README enlaza a docs/deploy.md
verify: cat docs/deploy.md | head -5
deps: S03 (ideal, no obligatorio)
status: done

## S05 | Bootstrap warning log
goal: exponer alerta visible si no hay admin creado
scope: cmd/aigw/main.go
steps:
  - Después de open DB, contar admin_users; si 0 → slog.Warn("no admin users — bootstrap at /admin/ or POST /admin/api/bootstrap")
done:
  - Log warning visible en arranque con DB vacía
  - Sin warning si ya hay admin
  - go vet ./... && go test ./... pasan
verify: go test ./...
deps: none
status: done
