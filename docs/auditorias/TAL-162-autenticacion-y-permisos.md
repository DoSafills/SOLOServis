# TAL-162 — Validación de autenticación y permisos

Fecha: 2026-11-12
Alcance: `backend/` — Auth API, middleware de autorización y rutas protegidas.

## 1. Controles verificados

| Control | Implementación | Evidencia |
|---|---|---|
| Contraseñas nunca en claro | PBKDF2 con sal, SHA-256 interno | `features/auth.go` (`passwordHash`, `checkPassword`) |
| Tokens opacos | 32 bytes aleatorios (`newToken`) | `features/auth.go` |
| Tokens hasheados en la base | SHA-256 antes del `INSERT` | `tokenHash`, tabla `user_session` |
| Expiración de access token | 15 minutos, `NOW()` de PostgreSQL | `issueTokens`, `authz.PostgresSessions` |
| Expiración de refresh token | 30 días | `issueTokens`, `Refresh` |
| Rotación al refrescar | `DELETE ... RETURNING user_id` + nuevo par | `Refresh` |
| Cierre de sesión | borra por hash de access **o** refresh | `Logout` |
| Rol administrativo | `RequireAdmin`: 401 sin token, 403 sin rol admin | `authz/authz.go` |
| Token de servicio | aceptado sólo si mide ≥ 32 caracteres | `authz.New` |
| Aislamiento por usuario | `RequireSession` inyecta la sesión; las consultas filtran por `user_id` | `authz.RequireSession`, handlers |
| Errores sin filtrar la base | 5xx genérico, detalle sólo al log | `authz_test.go` |

## 2. Pruebas

| Archivo | Qué fija |
|---|---|
| `features/auth_test.go` | register, login, refresh, logout, validate; duplicados, reglas, errores internos |
| `features/auth_credentials_test.go` | **nuevo**: los tokens se guardan como hash; logout consulta por hash |
| `authz/authz_test.go` | admin, cliente, token de servicio, token corto, sin header, `WWW-Authenticate` |
| `integration/postgres_test.go` | login real, validate real, contraseña incorrecta, email duplicado |
| `k8s/tests/smoke-test.sh` | rutas protegidas responden 401 dentro del clúster |

## 3. Hallazgos

- No se encontró ningún camino que guarde o registre un token en claro.
- El token de servicio corto se ignora con un warning, no se acepta.
- Las rutas de escritura distinguen 401 (sin sesión) de 403 (sesión sin rol).
- El logout es idempotente: sin token válido responde 204 y no revela si la sesión existía.

## 4. Riesgos residuales

| Riesgo | Recomendación |
|---|---|
| No hay rate limit específico en `/auth/login` | Añadir límite por IP/email para frenar fuerza bruta |
| El refresh token viaja en el body | Aceptable con HTTPS; evaluar cookie `HttpOnly` si el frontend lo permite |
| Las sesiones no se limpian por antigüedad | Job periódico que borre sesiones vencidas |

Ninguno bloquea la tarea: son endurecimientos posteriores.

## 5. Cómo reproducir

```bash
cd backend
go test ./internal/features/... -run "Auth|Session|Logout" -v
go test ./internal/authz/... -v
go test ./...
```

## 6. Conclusión

Autenticación y permisos están implementados y cubiertos por pruebas en tres
niveles: unidad, handler y smoke test en clúster. Las credenciales se manejan
hasheadas de extremo a extremo.
