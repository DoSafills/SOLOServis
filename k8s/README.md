# SOLOServis en Kubernetes (MVP)

```
Browser -> Ingress (/) -> frontend (nginx :8080) -> /products -> product-api (:8081)
                                                 -> /stores   -> store-api   (:8082)
product-api, store-api -> postgres:5432 (StatefulSet + PVC)
```

La plataforma UCT solo admite el path `/` por hostname en un Ingress
(ValidatingAdmissionPolicy `rke2-edu-ingress-hostname`). Por eso el enrutamiento
por path lo hace el nginx del frontend (`docker/frontend/nginx.conf`) hacia los
Services internos. El navegador solo usa el host público (mismo origen, sin CORS)
y el frontend se compila con `VITE_*_API_URL=""`.

## Estructura

- `base/` — manifests versionados (el namespace real lo fija el overlay según tu kubeconfig).
  - `postgres/` Secret (credenciales de desarrollo), Service ClusterIP + headless, StatefulSet.
  - `migrations/` Job único que aplica `backend/database/migrations/*.sql` una vez cada una
    (tabla `schema_migrations`).
  - `product-api/`, `store-api/`, `frontend/` Deployment + Service ClusterIP.
  - `ingress.yaml`.
- `overlays/local/` — **generado** por `scripts/k8s/deploy.ps1` (ignorado por git):
  registry/tag de imágenes, StorageClass validada, IngressClass, host y los `.sql`.

## Desplegar (Windows / PowerShell)

Requisitos: `kubectl`, `docker` con `docker login` al registry, `git`.

```powershell
# Solo inventario, no modifica nada
.\scripts\k8s\deploy.ps1 -Kubeconfig "$HOME\Downloads\estudiantes-<usuario>.kubeconfig" -ToPhase 0

# Todo (fases 0-10, con checkpoint por fase)
.\scripts\k8s\deploy.ps1 -Kubeconfig "$HOME\Downloads\estudiantes-<usuario>.kubeconfig" `
    -Registry docker.io/<usuario-docker>
```

Cada estudiante usa **su propio** kubeconfig (portal https://estudiantes.dev.censei.cl);
el namespace `student-<usuario>` se toma del kubeconfig y el host por defecto es
`soloservis-<usuario>.dev.censei.cl`. El Ingress usa la clase `nginx` y la anotación
`external-dns.alpha.kubernetes.io/target=proxy.inf.uct.cl` que exige la plataforma.


Parámetros útiles: `-FromPhase/-ToPhase`, `-StorageClass`, `-IngressClass`,
`-ImagePullSecret` (registry privado), `-SkipCleanup` (no borra nada en la Fase 1).

La Fase 1 solo borra recursos con `app.kubernetes.io/part-of=soloservis` y los nombres
heredados de la versión anterior (`statefulset/postgres`, `pvc/storage-test`, ...).
El PVC de PostgreSQL se borra solo si **no** está `Bound`.

## Fuera del MVP

auth, cart, comparison, favorites, ingestion, notification, pricing, review, search,
service, user y watchlist se agregan después como Deployments independientes,
siguiendo el patrón de `base/product-api/`.
