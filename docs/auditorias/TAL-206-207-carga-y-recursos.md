# TAL-206 y TAL-207 — Pruebas de carga y consumo de recursos

**Ejecución:** 2026-11-23, imágenes `7832154` en UCT y en local.

TAL-206 pide simular varios usuarios y solicitudes simultáneas; TAL-207,
monitorear CPU, memoria y uso de los servicios. Se hicieron juntas: el consumo
se mide mientras corre la carga.

## 1. Qué se agregó

- `scripts/testing/carga.py`: usuarios virtuales que navegan como el frontend
  (lista de productos, a veces filtrada por categoría; categorías; búsqueda;
  detalle; tiendas) con 1 a 3 s entre páginas. Sube por escalones y reporta por
  escalón y por ruta p50/p95/p99, errores por código y pedidos por segundo.
  Cada pedido lleva `X-Request-ID: carga-…`, que `errores.py` ya no cuenta como
  error real.
- `scripts/testing/recursos.py`: CPU y memoria de cada pod contra sus requests,
  limits y la cuota. Se puede correr solo o dentro de `reporte.py --recursos`.
- `carga.py --local`: levanta contenedores desechables `soloservis-carga-*`
  (PostgreSQL con las migraciones, comercial, usuarios y el gateway) con los
  mismos limits que en el clúster, y los borra al terminar.

Frenos contra UCT, que es compartido: tope de 15 r/s, y corte si en 15 s los
errores pasan del 20 %, el p95 pasa de 5 s o un pod se reinicia.

## 2. Cómo se mide el consumo en UCT

`kubectl top` no funciona: el rol del estudiante no puede listar pods en
`metrics.k8s.io` (Forbidden). En cambio, todos los pods dejan leer su cgroup v2,
así que `recursos.py` abre un `kubectl exec` por pod con un bucle de shell que
cada 5 s lee `cpu.stat`, `memory.current`, `memory.stat` y `memory.peak`. El
bucle usa solo comandos internos del shell (más `sleep`) para no sumarle CPU al
contenedor que mide.

- CPU: `usage_usec` entre dos muestras, en milicores, y el porcentaje de
  periodos en que el limit frenó al contenedor (throttling).
- Memoria: working set (`memory.current` menos `inactive_file`, lo que mira
  Kubernetes para un OOMKill) y el pico desde que arrancó.

## 3. Carga en UCT

Por el host público, con el certificado de la plataforma sin verificar:
5, 10 y 20 usuarios, 2 minutos cada escalón y 30 s de pausa. Reporte completo:
[`2026-11-23-0233-uct.md`](../reportes/carga/2026-11-23-0233-uct.md).

| Usuarios | r/s | p50 | p95 | p99 | Errores |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 5 | 2,4 | 38 ms | 91 ms | 212 ms | 0 |
| 10 | 4,9 | 36 ms | 92 ms | 208 ms | 0 |
| 20 | 9,5 | 35 ms | 80 ms | 195 ms | 0 |

2006 pedidos, ningún 5xx, timeout ni 429, y ningún reinicio. Hubo 8 respuestas
400 en `/products?category`: el script tomaba los filtros de `/categories`, que
también trae categorías de servicios. Se corrigió para usar
`/product-categories`, como el frontend; no es un error del backend.

Consumo con 20 usuarios (el escalón más alto):

- comercial: 27m de CPU (9 % de su limit de 300m), 9Mi de memoria.
- gateway (frontend): 7m de 100m, 13Mi.
- postgres: 44m de 250m, 48Mi. En reposo ya usa unos 30m y muestra throttling
  en un tercio de sus periodos: son picos cortos, no saturación.
- usuarios, ingestion y scraper-api no cambian con la carga.

La cuota es lo que más aprieta, y es por lo reservado, no por lo consumido:
`limits.memory` va en 1664Mi de 2G (87 %) y `limits.cpu` en 1550m de 2 (78 %),
mientras el consumo real sumado de los pods fue 65m de CPU y 121Mi de memoria.

## 4. Carga fuerte en local

En UCT no se pasó de 20 usuarios. Para ver dónde se satura se usaron los
contenedores desechables, con los limits del clúster.

**Directo a comercial, sin espera ni tope** (20, 50 y 100 usuarios, 1 minuto
cada uno;
[`2026-11-23-0251-local-directo.md`](../reportes/carga/2026-11-23-0251-local-directo.md)):
290, 313 y 376 r/s, con p95 de 107, 302 y 499 ms y cero errores. El cuello de
botella es PostgreSQL: llegó a su limit de 250m y estuvo frenado en el 96 % de
los periodos, mientras comercial quedó en 173m de 300m.

**Por el gateway, 50 usuarios sin espera** (30 s, sin frenos;
[`2026-11-23-0256-local.md`](../reportes/carga/2026-11-23-0256-local.md)): de
~900 pedidos por segundo pasaron 961 en 30 s, unos 32 por segundo; el resto
recibió 429 `rate_limited`. El límite de TAL-201 (30 r/s por IP, ráfaga 60)
funciona. Para responder esos 429 el gateway usó 65m de media y 99m de máximo,
con throttling en el 93 % de los periodos: su limit de 100m es el techo del
sitio si llega tráfico de muchas IPs a la vez.

**Mismo recorrido que en UCT, con login** (5, 10 y 20 usuarios;
[`2026-11-23-0258-local.md`](../reportes/carga/2026-11-23-0258-local.md)):
cero errores; las lecturas responden en p95 de 3 a 7 ms. El login es lo caro:
p50 144 ms y p95 1,5 s, porque el PBKDF2 pasa por dos cupos en el proceso
`usuarios` (TAL-201), que llega a 108m de sus 200m. De 66 intentos, 31 fueron
429 del gateway (20 logins por minuto por IP; todos los usuarios virtuales
salen de la misma) y entraron al reintentar.

En una primera corrida todos los usuarios virtuales compartían una cuenta y
`/auth/validate` devolvió 86 veces 401: el backend deja 10 sesiones vigentes
por usuario (TAL-200) y cada login nuevo invalidaba el token más antiguo. Es el
comportamiento esperado; el script ahora usa una cuenta cada 5 usuarios.

## 5. Conclusiones

- Con 20 usuarios simultáneos el sitio en UCT responde en p95 bajo 100 ms y
  usa menos del 10 % de la CPU de comercial: hay margen de sobra para el uso
  del curso.
- El primer recurso que se agota es la CPU de PostgreSQL (250m), cerca de los
  300 r/s de lecturas; después, la del gateway (100m).
- El límite real del namespace es la cuota de limits de memoria (87 %), no el
  consumo. Si se necesita subir CPU de postgres o del gateway, hay espacio en
  `limits.cpu` (quedan 450m) pero conviene revisar antes la memoria reservada.

## 6. Cómo repetirlo

```bash
python scripts/testing/recursos.py --duracion 60             # solo consumo, sin carga
python scripts/testing/carga.py --sin-verificar-tls          # UCT, pedir aprobación antes
python scripts/testing/carga.py --local --login
python scripts/testing/carga.py --local --directo --escalones 20,50,100 --espera 0 --max-rps 0 --duracion 60 --pausa 15
python scripts/testing/carga.py --local --escalones 50 --espera 0 --max-rps 0 --sin-frenos --duracion 30 --pausa 10
```
