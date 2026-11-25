---
name: k8s-deploy
description: Use when deploying to the UCT cluster, changing k8s/ manifests, editing deploy.ps1 or deploy.sh, running the smoke test Job, or debugging a pod that will not start. Covers the 11 phases, quota, secrets and Pod Security.
---

# Despliegue — `k8s/`

Guía completa: `k8s/README.md`. Reglas generales en el `AGENTS.md` de la raíz
(§11).

## Modelo

- **14 APIs en 3 dominios** (`comercial`, `usuarios`, `ingestion`) con una sola
  imagen; `API_DOMAIN` decide. Topología alternativa `consolidado` en un solo
  Deployment.
- **El API Gateway es el nginx del frontend.** Sólo el frontend se publica.
- `k8s/base/` es la base; `k8s/components/{dominios,consolidado,espera-postgres}`
  los componentes; `k8s/overlays/` **se genera y está ignorado por git**.

## Las 11 fases

| Fase | Qué hace |
| --- | --- |
| 0 | Inventario: herramientas, acceso, cuota. Sólo lectura |
| 1 | Borra el despliegue anterior; **conserva PVC y Secrets** |
| 2 | Elige StorageClass y genera `k8s/overlays/uct` |
| 3 | `docker build` + `docker push` de frontend, backend y scraper |
| 4 | Crea `postgres-secret`, `soloservis-internal`, `soloservis-scraper` y `soloservis-ai` si faltan; despliega PostgreSQL |
| 5 | Job de migraciones según `migration_order.txt` |
| 6 | APIs + `scraper-api` (`/health` con `database: connected`) |
| 7 | Frontend (nginx) |
| 8 | Ingress |
| 9 | Resiliencia: borra un pod, reinicia PostgreSQL |
| 10 | Pod de test (`k8s/tests`) |

## Comandos

```bash
./scripts/k8s/deploy.sh --modo uct --registry docker.io/<usuario>
./scripts/k8s/deploy.sh --modo uct --to-phase 0                 # sólo inventario
./scripts/k8s/deploy.sh --modo uct --from-phase 3 --skip-cleanup # redeploy sin borrar datos
./scripts/k8s/deploy.sh --modo uct --from-phase 10              # sólo el pod de test
```

El tag por defecto es el commit actual (`git rev-parse --short HEAD`). Sin
`--registry` el script se detiene en la Fase 3.

## Reglas que rompen el despliegue

1. **Cuota del namespace:** 12 pods, 10 Services, 3 PVC, 1 CPU / 1G de requests.
2. **LimitRange:** mínimo **25m y 64Mi** por contenedor, también en Jobs y
   CronJobs.
3. **Pod Security `restricted`:** sin root, `readOnlyRootFilesystem`, sin
   capabilities.
4. **`enableServiceLinks: false`** si una variable `<SERVICIO>_PORT` choca con
   la configuración de la app.
5. **`deploy.ps1` en ASCII** (PowerShell 5.1) y **`deploy.sh` compatible con
   bash 3.2**. Si cambias uno, cambias el otro: son el mismo flujo.
6. **Los Secrets no están en git.** Se nombran, nunca se imprimen.

## Pod de test

El Job `soloservis-smoke-test` (`k8s/tests`) tiene tres contenedores en un pod:

- `smoke-test` corre `k8s/tests/smoke-test.sh` con `curlimages/curl`. Cubre
  APIs, frontend, aislamiento entre dominios y, si `scraper-api` existe, sus
  `/health`, `/ready`, `/stores` y `/runs`.
- `origen` y el sidecar nativo `destino` son el visor de llamadas
  (TAL-165/166/167): `origen.py` envía, `destino.py` muestra lo que recibe cada
  servicio. Escenarios en `escenarios.py` y `flujo.py`.

Sólo lee: los POST van sin token o con bodies que se rechazan antes de tocar
la base. El Job pasa solo si `smoke-test` y `origen` salen con 0.

```bash
kubectl apply -k k8s/tests
kubectl wait --for=condition=complete job/soloservis-smoke-test --timeout=600s
kubectl logs job/soloservis-smoke-test -c smoke-test
kubectl logs -f job/soloservis-smoke-test -c origen    # ventana 1: lo enviado
kubectl logs -f job/soloservis-smoke-test -c destino   # ventana 2: lo recibido
```

`python scripts/testing/llamadas.py --ventanas` relanza el Job, abre las dos
ventanas y deja el reporte lado a lado en `docs/reportes/testing/`.

## Verificación sin desplegar

```bash
kubectl kustomize k8s/overlays/uct
bash scripts/k8s/validate-manifests.sh   # esquema de la API, sin tocar el clúster
docker compose -f docker/compose.yml config --quiet
sh -n k8s/tests/smoke-test.sh
python -m py_compile k8s/tests/origen.py k8s/tests/destino.py k8s/tests/escenarios.py k8s/tests/flujo.py
sh -n scripts/k8s/deploy.sh
```

## Ante un pod que no arranca

`kubectl describe pod <pod>` → eventos; `kubectl logs <pod> --previous` si
reinició; revisar cuota y LimitRange antes de tocar el manifiesto. Un
`ImagePullBackOff` es registry o `imagePullSecret`, no código.

## Anti-patrones

- Editar `k8s/overlays/` a mano: se regenera.
- Cambiar sólo `deploy.sh` y dejar `deploy.ps1` atrás.
- Subir la cuota en vez de ajustar los requests.
- Guardar un Secret en git o imprimirlo en un log.
