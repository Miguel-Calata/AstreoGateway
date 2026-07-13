# Deploy — AstreoGateway

Runbook de producción. Alcance: un binario en Docker, SQLite en volume,
TLS en reverse proxy. No hay YAML de config de app; el estado vive en la DB.

## 1. Requisitos

- Docker + Compose v2
- Puerto libre (default host `18473`, o remap)
- HTTPS recomendado en producción (cookie admin + tráfico `/v1/*`)

## 2. Arranque (compose)

```bash
docker compose up --build -d
curl -s http://localhost:18473/healthz
```

Respuesta esperada: JSON con `"status":"ok"` (HTTP 200). Si la DB está vacía,
en logs aparece:

```text
no admin users — bootstrap at /admin/ or POST /admin/api/bootstrap
```

Persistencia: volume named `aigw-data` → `/app/data` (`-db /app/data/aigw.db`).

Alternativa sin compose:

```bash
docker build -t aigw .
docker run -d --name aigw -p 18473:18473 -v aigw-data:/app/data \
  -e AIGW_LOG_LEVEL=info aigw
```

## 3. Bootstrap

La primera vez no hay admin. Tras crear el primero, bootstrap queda cerrado.

### UI

1. Abrir `https://<host>/admin/` (o `http://localhost:18473/admin/` en local).
2. Crear usuario admin.
3. Login → Providers, API Keys, Aliases, Gateway Keys, Discovery.

### API (headless)

```bash
# 1. Primer admin (solo si no hay usuarios)
curl -s -X POST http://localhost:18473/admin/api/bootstrap \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<password-fuerte>"}'

# 2. Login (cookie de sesión)
curl -s -c cookies.txt -X POST http://localhost:18473/admin/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<password-fuerte>"}'

# 3. CRUD autenticado con -b cookies.txt
#    POST /admin/api/providers
#    POST /admin/api/providers/{id}/api-keys
#    POST /admin/api/gateway-keys
#    POST /admin/api/aliases
```

Smoke del gateway público (tras crear una gateway key):

```bash
curl -s http://localhost:18473/v1/models \
  -H "Authorization: Bearer <gateway_key>"
```

## 4. Reverse proxy + cookie Secure

Terminar TLS en el proxy. El contenedor escucha HTTP en `:18473`.

**Obligatorio detrás de HTTPS:**

```yaml
# docker-compose.yml
environment:
  AIGW_COOKIE_SECURE: "true"
```

Sin `AIGW_COOKIE_SECURE=true`, la cookie de sesión admin no lleva flag `Secure`.

### Caddy

```caddy
aigw.example.com {
	reverse_proxy 127.0.0.1:18473
}
```

### nginx

```nginx
server {
	listen 443 ssl http2;
	server_name aigw.example.com;

	# ssl_certificate / ssl_certificate_key ...

	location / {
		proxy_pass http://127.0.0.1:18473;
		proxy_http_version 1.1;
		proxy_set_header Host $host;
		proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
		proxy_set_header X-Forwarded-Proto $scheme;
		proxy_read_timeout 3600s;
		proxy_send_timeout 3600s;
	}
}
```

Los timeouts largos evitan cortar streams de chat en el proxy (el server del
gateway usa `WriteTimeout: 0`).

Si compose no publica el puerto al host, apunta el proxy al servicio en la red
Docker (`http://aigw:18473`) en lugar de `127.0.0.1:18473`.

## 5. Permisos de volume (nonroot)

Runtime: distroless `USER nonroot` (UID/GID **65532**).

| Montaje | Comportamiento |
|---------|----------------|
| Named volume (`aigw-data`) | Suele heredar ownership del dir imagen (`/app/data` chown 65532) → OK |
| Bind mount host | Hay que alinear UID o fallará escritura de la DB |

Bind mount:

```bash
mkdir -p ./data
sudo chown -R 65532:65532 ./data
```

```yaml
# docker-compose.yml (ejemplo)
volumes:
  - ./data:/app/data
```

Síntoma típico de permisos mal: error al abrir/ping SQLite en arranque.

## 6. Health check externo

La imagen no tiene shell ni `curl` → no hay `HEALTHCHECK` in-image.

Probe desde el host u orquestador:

```bash
curl -sf http://localhost:18473/healthz
```

`/healthz` hace ping a la DB y devuelve uptime. HTTP 200 = listo; 503 = DB caída.

## 7. Backup / restore SQLite

Ruta en contenedor: `/app/data/aigw.db`. Journal **WAL** activo: pueden existir
`aigw.db-wal` y `aigw.db-shm`.

**Preferido (copia en frío):**

```bash
docker compose stop
# Named volume: copiar con un contenedor auxiliar
docker run --rm -v aigw-data:/data -v "$(pwd)/backup:/backup" alpine \
  tar czf /backup/aigw-$(date +%Y%m%d%H%M).tgz -C /data .
docker compose start
```

**Restore:**

```bash
docker compose stop
docker run --rm -v aigw-data:/data -v "$(pwd)/backup:/backup" alpine \
  sh -c 'rm -rf /data/* && tar xzf /backup/aigw-YYYYMMDDHHMM.tgz -C /data'
docker compose start
curl -sf http://localhost:18473/healthz
```

Con bind mount, basta copiar el directorio host (`./data`) con el stack parado.

**Seguridad:** el backup incluye secrets de proveedor en claro en SQLite.
Tratar backups como secretos.

## 8. Upgrade de imagen

```bash
# Rebuild local
docker compose build --pull
docker compose up -d

# O pull si publicas a un registry y usas image: ...
# docker compose pull && docker compose up -d
```

El volume se reutiliza. Al arrancar, `store.Migrate` aplica migraciones
pendientes. Antes de un upgrade mayor, revisar `internal/store/migrations/`.

Rollback: arrancar imagen anterior con el mismo volume (solo si las migraciones
aplicadas son compatibles hacia atrás).

## 9. Variables de entorno

| Variable | Default | Producción |
|----------|---------|------------|
| `AIGW_LOG_LEVEL` | `info` | `info` o `warn` |
| `AIGW_COOKIE_SECURE` | `false` | `true` detrás de HTTPS |
| `AIGW_ADDR` | `:18473` (CMD imagen) | raramente hace falta |
| `AIGW_DB` | `/app/data/aigw.db` (CMD) | mantener path en volume |
| `AIGW_DISCOVERY_TTL` | `5m` | tunear si hace falta |
| `AIGW_DISCOVERY_TIMEOUT` | `10s` | |
| `AIGW_PROXY_TIMEOUT` | `120s` | streams largos |
| `AIGW_KEY_COOLDOWN` | `30s` | tras 429/5xx |

Flags equivalentes: ver README (`-addr`, `-db`, `-cookie-secure`, …).

## 10. Checklist mínimo

- [ ] `docker compose up -d` y `GET /healthz` → 200
- [ ] Bootstrap admin hecho (sin warning de “no admin users” en logs)
- [ ] TLS en reverse proxy + `AIGW_COOKIE_SECURE=true`
- [ ] Volume persistente: `compose restart` mantiene datos
- [ ] Backup en frío probado una vez (y restaurado en staging si es posible)
- [ ] Provider + API key upstream + gateway key
- [ ] Smoke: `GET /v1/models` o `POST /v1/chat/completions` con Bearer
- [ ] Contraseñas/keys no commiteadas; backups tratados como secretos

## 11. Known issues de producción

- Secrets de proveedor en claro en SQLite (`api_keys.key_value`).
- Sin métricas ricas: solo `GET /healthz` (RPS/errores pendientes).
- Runtime distroless: sin shell para debug in-container; inspeccionar desde fuera
  o con contenedor auxiliar montando el mismo volume.
